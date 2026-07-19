package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// load points ENV_FILE at a temp .env with the given content and runs Load,
// clearing the keys the file sets afterwards so tests stay independent.
func load(t *testing.T, content string, keys ...string) (Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV_FILE", path)
	t.Cleanup(func() {
		for _, k := range keys {
			os.Unsetenv(k)
		}
	})
	return Load()
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != Default() {
		t.Errorf("Load() with no env = %+v, want defaults", cfg)
	}
}

func TestLoadDotenv(t *testing.T) {
	cfg, err := load(t, `
# comment
ADDR=127.0.0.1:9999
export DB_PATH="/data/tp.db"
SESSION_TTL_DAYS=7
LOGIN_RATE_WINDOW=30s
COOKIE_SECURE=false
SET_MIN_WORDS=10
SET_MAX_WORDS=20
`, "ADDR", "DB_PATH", "SESSION_TTL_DAYS", "LOGIN_RATE_WINDOW", "COOKIE_SECURE", "SET_MIN_WORDS", "SET_MAX_WORDS")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9999" || cfg.DBPath != "/data/tp.db" {
		t.Errorf("addr/db not applied: %+v", cfg)
	}
	if cfg.SessionTTL != 7*24*time.Hour {
		t.Errorf("SessionTTL = %v, want 168h", cfg.SessionTTL)
	}
	if cfg.LoginRateWindow != 30*time.Second || cfg.CookieSecure {
		t.Errorf("rate window / cookie not applied: %+v", cfg)
	}
	if cfg.SetMinWords != 10 || cfg.SetMaxWords != 20 {
		t.Errorf("set range not applied: %+v", cfg)
	}
}

func TestRealEnvWins(t *testing.T) {
	t.Setenv("ADDR", ":7000")
	cfg, err := load(t, "ADDR=:9000\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":7000" {
		t.Errorf("Addr = %q, want real env to win over .env", cfg.Addr)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name, content, wantSub string
		keys                   []string
	}{
		{"malformed line", "NOT A PAIR\n", "expected KEY=value", nil},
		{"bad int", "LOGIN_RATE_LIMIT=lots\n", "LOGIN_RATE_LIMIT", []string{"LOGIN_RATE_LIMIT"}},
		{"bad duration", "READ_TIMEOUT=fast\n", "READ_TIMEOUT", []string{"READ_TIMEOUT"}},
		{"bad set range", "SET_MIN_WORDS=50\nSET_MAX_WORDS=40\n", "SET_MIN_WORDS", []string{"SET_MIN_WORDS", "SET_MAX_WORDS"}},
		{"bad base path", "BASE_PATH=typing\n", "BASE_PATH", []string{"BASE_PATH"}},
		{"bad log format", "LOG_FORMAT=xml\n", "LOG_FORMAT", []string{"LOG_FORMAT"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(t, c.content, c.keys...)
			if err == nil || !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("err = %v, want mention of %q", err, c.wantSub)
			}
		})
	}
}
