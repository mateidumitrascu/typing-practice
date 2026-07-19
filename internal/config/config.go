// Package config centralizes all server configuration. Values come from the
// environment, optionally seeded from a .env file (real env vars win). The
// file format is plain KEY=value, compatible with systemd's EnvironmentFile.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Addr     string
	BasePath string
	DBPath   string

	LogLevel  slog.Level
	LogFormat string // text | json

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	SessionTTL      time.Duration
	LoginRateLimit  int
	LoginRateWindow time.Duration
	CookieSecure    bool
	IPHeader        string
	BcryptCost      int

	SetMinWords int
	SetMaxWords int

	ResultsDefaultLimit int
	ResultsMaxLimit     int
	StatsSeriesLimit    int
}

func Default() Config {
	return Config{
		Addr:     ":8080",
		BasePath: "",
		DBPath:   "typepractice.db",

		LogLevel:  slog.LevelInfo,
		LogFormat: "text",

		ReadTimeout:     10 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     2 * time.Minute,
		ShutdownTimeout: 10 * time.Second,

		SessionTTL:      30 * 24 * time.Hour,
		LoginRateLimit:  5,
		LoginRateWindow: time.Minute,
		CookieSecure:    true,
		IPHeader:        "CF-Connecting-IP",
		BcryptCost:      bcrypt.DefaultCost,

		SetMinWords: 30,
		SetMaxWords: 45,

		ResultsDefaultLimit: 20,
		ResultsMaxLimit:     500,
		StatsSeriesLimit:    100,
	}
}

// Load seeds the environment from ENV_FILE (default ".env", ignored if
// missing), then parses and validates the full configuration.
func Load() (Config, error) {
	file := ".env"
	if v, ok := os.LookupEnv("ENV_FILE"); ok {
		file = v
	}
	if err := loadDotenv(file); err != nil {
		return Config{}, err
	}

	cfg := Default()
	p := &parser{}

	p.str("ADDR", &cfg.Addr)
	p.str("BASE_PATH", &cfg.BasePath)
	p.str("DB_PATH", &cfg.DBPath)

	p.level("LOG_LEVEL", &cfg.LogLevel)
	p.str("LOG_FORMAT", &cfg.LogFormat)

	p.dur("READ_TIMEOUT", &cfg.ReadTimeout)
	p.dur("WRITE_TIMEOUT", &cfg.WriteTimeout)
	p.dur("IDLE_TIMEOUT", &cfg.IdleTimeout)
	p.dur("SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout)

	p.days("SESSION_TTL_DAYS", &cfg.SessionTTL)
	p.num("LOGIN_RATE_LIMIT", &cfg.LoginRateLimit)
	p.dur("LOGIN_RATE_WINDOW", &cfg.LoginRateWindow)
	p.boolean("COOKIE_SECURE", &cfg.CookieSecure)
	p.str("IP_HEADER", &cfg.IPHeader)
	p.num("BCRYPT_COST", &cfg.BcryptCost)

	p.num("SET_MIN_WORDS", &cfg.SetMinWords)
	p.num("SET_MAX_WORDS", &cfg.SetMaxWords)

	p.num("RESULTS_DEFAULT_LIMIT", &cfg.ResultsDefaultLimit)
	p.num("RESULTS_MAX_LIMIT", &cfg.ResultsMaxLimit)
	p.num("STATS_SERIES_LIMIT", &cfg.StatsSeriesLimit)

	if p.err != nil {
		return Config{}, p.err
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	switch {
	case c.LogFormat != "text" && c.LogFormat != "json":
		return fmt.Errorf("LOG_FORMAT must be text or json, got %q", c.LogFormat)
	case c.SessionTTL <= 0:
		return fmt.Errorf("SESSION_TTL_DAYS must be positive")
	case c.LoginRateLimit < 1:
		return fmt.Errorf("LOGIN_RATE_LIMIT must be at least 1")
	case c.LoginRateWindow <= 0:
		return fmt.Errorf("LOGIN_RATE_WINDOW must be positive")
	case c.BcryptCost < bcrypt.MinCost || c.BcryptCost > bcrypt.MaxCost:
		return fmt.Errorf("BCRYPT_COST must be %d..%d", bcrypt.MinCost, bcrypt.MaxCost)
	case c.SetMinWords < 1 || c.SetMaxWords < c.SetMinWords:
		return fmt.Errorf("SET_MIN_WORDS..SET_MAX_WORDS must be a valid range, got %d..%d", c.SetMinWords, c.SetMaxWords)
	case c.ResultsDefaultLimit < 1 || c.ResultsMaxLimit < c.ResultsDefaultLimit:
		return fmt.Errorf("RESULTS_DEFAULT_LIMIT..RESULTS_MAX_LIMIT must be a valid range")
	case c.StatsSeriesLimit < 2:
		return fmt.Errorf("STATS_SERIES_LIMIT must be at least 2")
	case c.BasePath != "" && !strings.HasPrefix(c.BasePath, "/"):
		return fmt.Errorf("BASE_PATH must start with / (got %q)", c.BasePath)
	}
	return nil
}

// loadDotenv sets variables from a KEY=value file. Variables already present
// in the environment are never overridden. A missing file is not an error.
func loadDotenv(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=value, got %q", path, i+1, line)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return nil
}

// parser reads typed env vars, keeping the first error it hits.
type parser struct{ err error }

func (p *parser) lookup(key string) (string, bool) {
	if p.err != nil {
		return "", false
	}
	v, ok := os.LookupEnv(key)
	return v, ok
}

func (p *parser) str(key string, dst *string) {
	if v, ok := p.lookup(key); ok {
		*dst = v
	}
}

func (p *parser) num(key string, dst *int) {
	if v, ok := p.lookup(key); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			p.err = fmt.Errorf("%s: expected an integer, got %q", key, v)
			return
		}
		*dst = n
	}
}

func (p *parser) boolean(key string, dst *bool) {
	if v, ok := p.lookup(key); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			p.err = fmt.Errorf("%s: expected true or false, got %q", key, v)
			return
		}
		*dst = b
	}
}

func (p *parser) dur(key string, dst *time.Duration) {
	if v, ok := p.lookup(key); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			p.err = fmt.Errorf("%s: expected a duration like 30s or 2m, got %q", key, v)
			return
		}
		*dst = d
	}
}

func (p *parser) days(key string, dst *time.Duration) {
	if v, ok := p.lookup(key); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			p.err = fmt.Errorf("%s: expected a number of days, got %q", key, v)
			return
		}
		*dst = time.Duration(n) * 24 * time.Hour
	}
}

func (p *parser) level(key string, dst *slog.Level) {
	if v, ok := p.lookup(key); ok {
		if err := dst.UnmarshalText([]byte(v)); err != nil {
			p.err = fmt.Errorf("%s: expected debug, info, warn or error, got %q", key, v)
		}
	}
}
