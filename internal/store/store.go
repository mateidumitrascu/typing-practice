package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS results (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  mode TEXT NOT NULL,
  letter TEXT,
  wpm REAL NOT NULL, net_wpm REAL NOT NULL, accuracy REAL NOT NULL,
  word_count INTEGER NOT NULL, duration_ms INTEGER NOT NULL,
  chars_typed INTEGER NOT NULL, errors INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_results_user_time ON results(user_id, created_at);
CREATE TABLE IF NOT EXISTS settings (
  user_id INTEGER NOT NULL REFERENCES users(id),
  client TEXT NOT NULL,
  theme TEXT NOT NULL,
  PRIMARY KEY (user_id, client)
);
`

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	// modernc.org/sqlite serializes writes; a single connection avoids
	// SQLITE_BUSY under concurrent access
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	return err
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, userID, expiresAt.UTC().Format(time.RFC3339))
	return err
}

// SessionUser resolves a token hash to a user ID, treating expired sessions as missing.
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	var expiresAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(exp) {
		return 0, ErrNotFound
	}
	return userID, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}

type Result struct {
	ID         int64   `json:"id"`
	Mode       string  `json:"mode"`
	Letter     string  `json:"letter,omitempty"`
	WPM        float64 `json:"wpm"`
	NetWPM     float64 `json:"net_wpm"`
	Accuracy   float64 `json:"accuracy"`
	WordCount  int     `json:"word_count"`
	DurationMs int64   `json:"duration_ms"`
	CharsTyped int     `json:"chars_typed"`
	Errors     int     `json:"errors"`
	CreatedAt  string  `json:"created_at"`
}

func (s *Store) InsertResult(ctx context.Context, userID int64, r Result) error {
	var letter any
	if r.Letter != "" {
		letter = r.Letter
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO results (user_id, mode, letter, wpm, net_wpm, accuracy, word_count, duration_ms, chars_typed, errors)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, r.Mode, letter, r.WPM, r.NetWPM, r.Accuracy, r.WordCount, r.DurationMs, r.CharsTyped, r.Errors)
	return err
}

func (s *Store) Results(ctx context.Context, userID int64, limit int) ([]Result, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, mode, COALESCE(letter, ''), wpm, net_wpm, accuracy, word_count, duration_ms, chars_typed, errors, created_at
		 FROM results WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Result{}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.Mode, &r.Letter, &r.WPM, &r.NetWPM, &r.Accuracy,
			&r.WordCount, &r.DurationMs, &r.CharsTyped, &r.Errors, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Stats struct {
	TotalTests int      `json:"total_tests"`
	BestWPM    float64  `json:"best_wpm"`
	BestNetWPM float64  `json:"best_net_wpm"`
	AvgWPM10   float64  `json:"avg_wpm_last_10"`
	AvgAcc10   float64  `json:"avg_accuracy_last_10"`
	Series     []Result `json:"series"`
}

// UserStats returns aggregates plus the last `seriesLimit` results in
// chronological order for charting.
func (s *Store) UserStats(ctx context.Context, userID int64, seriesLimit int) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(wpm), 0), COALESCE(MAX(net_wpm), 0) FROM results WHERE user_id = ?`,
		userID).Scan(&st.TotalTests, &st.BestWPM, &st.BestNetWPM)
	if err != nil {
		return st, err
	}
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG(wpm), 0), COALESCE(AVG(accuracy), 0) FROM
		 (SELECT wpm, accuracy FROM results WHERE user_id = ? ORDER BY id DESC LIMIT 10)`,
		userID).Scan(&st.AvgWPM10, &st.AvgAcc10)
	if err != nil {
		return st, err
	}
	series, err := s.Results(ctx, userID, seriesLimit)
	if err != nil {
		return st, err
	}
	for i, j := 0, len(series)-1; i < j; i, j = i+1, j-1 {
		series[i], series[j] = series[j], series[i]
	}
	st.Series = series
	return st, nil
}

func (s *Store) Setting(ctx context.Context, userID int64, client string) (string, error) {
	var theme string
	err := s.db.QueryRowContext(ctx,
		`SELECT theme FROM settings WHERE user_id = ? AND client = ?`, userID, client).Scan(&theme)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return theme, err
}

func (s *Store) SetSetting(ctx context.Context, userID int64, client, theme string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (user_id, client, theme) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, client) DO UPDATE SET theme = excluded.theme`,
		userID, client, theme)
	return err
}
