package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mateidumitrascu/typing-practice/internal/config"
	"github.com/mateidumitrascu/typing-practice/internal/stats"
	"github.com/mateidumitrascu/typing-practice/internal/store"
	"github.com/mateidumitrascu/typing-practice/internal/theme"
	"github.com/mateidumitrascu/typing-practice/internal/words"
)

const sessionCookie = "session"

type Server struct {
	store    *store.Store
	gen      *words.Generator
	cfg      config.Config
	basePath string
	limiter  *rateLimiter
}

// New builds the full handler. cfg.BasePath ("" or "/typing") is where the app
// is mounted; webFS is the built web client, served at the base path root.
func New(st *store.Store, cfg config.Config, webFS fs.FS) http.Handler {
	gen := words.NewGenerator()
	gen.SetLengthRange(cfg.SetMinWords, cfg.SetMaxWords)
	s := &Server{
		store:    st,
		gen:      gen,
		cfg:      cfg,
		basePath: strings.TrimSuffix(cfg.BasePath, "/"),
		limiter:  newRateLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.auth(s.handleLogout))
	mux.HandleFunc("GET /api/words", s.handleWords)
	mux.HandleFunc("GET /api/themes", s.handleThemes)
	mux.HandleFunc("POST /api/results", s.auth(s.handleCreateResult))
	mux.HandleFunc("GET /api/results", s.auth(s.handleListResults))
	mux.HandleFunc("GET /api/stats", s.auth(s.handleStats))
	mux.HandleFunc("GET /api/settings/{client}", s.auth(s.handleGetSetting))
	mux.HandleFunc("PUT /api/settings/{client}", s.auth(s.handlePutSetting))
	mux.Handle("GET /", http.FileServerFS(webFS))

	if s.basePath == "" {
		return mux
	}
	outer := http.NewServeMux()
	outer.Handle(s.basePath+"/", http.StripPrefix(s.basePath, mux))
	outer.HandleFunc(s.basePath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.basePath+"/", http.StatusMovedPermanently)
	})
	return outer
}

type ctxKey int

const userIDKey ctxKey = 0

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		} else if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}
		if token == "" {
			jsonError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, err := s.store.SessionUser(r.Context(), hashToken(token))
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	}
}

func userID(r *http.Request) int64 {
	id, _ := r.Context().Value(userIDKey).(int64)
	return id
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(s.clientIP(r)) {
		jsonError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := s.store.UserByUsername(r.Context(), req.Username)
	if err == nil {
		err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password))
	}
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	raw := make([]byte, 32)
	rand.Read(raw)
	token := hex.EncodeToString(raw)
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.store.CreateSession(r.Context(), hashToken(token), u.ID, expires); err != nil {
		internalError(w, err)
		return
	}
	go s.store.DeleteExpiredSessions(context.Background())

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token,
		Path: s.basePath + "/", Expires: expires,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expires.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.store.DeleteSession(r.Context(), hashToken(c.Value))
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		s.store.DeleteSession(r.Context(), hashToken(strings.TrimPrefix(h, "Bearer ")))
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: s.basePath + "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	switch mode {
	case "", "speed", "free":
		writeJSON(w, http.StatusOK, map[string]any{"words": s.gen.Set()})
	case "letter":
		l := r.URL.Query().Get("letter")
		if len(l) != 1 || l[0] < 'a' || l[0] > 'z' {
			jsonError(w, http.StatusBadRequest, "letter must be a single character a-z")
			return
		}
		set, err := s.gen.LetterSet(rune(l[0]))
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"words": set, "letter": l})
	default:
		jsonError(w, http.StatusBadRequest, "mode must be speed, free or letter")
	}
}

func (s *Server) handleThemes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"themes":  theme.All(),
		"default": theme.Default().Name,
	})
}

func (s *Server) handleCreateResult(w http.ResponseWriter, r *http.Request) {
	var res store.Result
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if res.Mode != "speed" && res.Mode != "letter" {
		jsonError(w, http.StatusBadRequest, "mode must be speed or letter")
		return
	}
	if res.Mode == "letter" && len(res.Letter) != 1 {
		jsonError(w, http.StatusBadRequest, "letter mode requires a letter")
		return
	}
	if res.WordCount <= 0 || res.DurationMs <= 0 || res.CharsTyped <= 0 {
		jsonError(w, http.StatusBadRequest, "word_count, duration_ms and chars_typed must be positive")
		return
	}
	// recompute rather than trust client math
	elapsed := time.Duration(res.DurationMs) * time.Millisecond
	res.WPM = stats.GrossWPM(res.CharsTyped, elapsed)
	res.NetWPM = stats.NetWPM(res.CharsTyped, res.Errors, elapsed)
	if err := s.store.InsertResult(r.Context(), userID(r), res); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleListResults(w http.ResponseWriter, r *http.Request) {
	limit := s.cfg.ResultsDefaultLimit
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= s.cfg.ResultsMaxLimit {
			limit = n
		}
	}
	results, err := s.store.Results(r.Context(), userID(r), limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.UserStats(r.Context(), userID(r), s.cfg.StatsSeriesLimit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func validClient(c string) bool { return c == "tui" || c == "web" }

func (s *Server) handleGetSetting(w http.ResponseWriter, r *http.Request) {
	client := r.PathValue("client")
	if !validClient(client) {
		jsonError(w, http.StatusBadRequest, "client must be tui or web")
		return
	}
	t, err := s.store.Setting(r.Context(), userID(r), client)
	if errors.Is(err, store.ErrNotFound) {
		t = theme.Default().Name
	} else if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"theme": t})
}

func (s *Server) handlePutSetting(w http.ResponseWriter, r *http.Request) {
	client := r.PathValue("client")
	if !validClient(client) {
		jsonError(w, http.StatusBadRequest, "client must be tui or web")
		return
	}
	var req struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, ok := theme.Get(req.Theme); !ok {
		jsonError(w, http.StatusBadRequest, "unknown theme")
		return
	}
	if err := s.store.SetSetting(r.Context(), userID(r), client, req.Theme); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// clientIP prefers the configured proxy header (CF-Connecting-IP by default,
// since the app runs behind a Cloudflare tunnel).
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.IPHeader != "" {
		if ip := r.Header.Get(s.cfg.IPHeader); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func internalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "err", err)
	jsonError(w, http.StatusInternalServerError, "internal error")
}
