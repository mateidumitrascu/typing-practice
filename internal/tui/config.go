package tui

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultServer is where the TUI points when nothing else is configured.
// Release builds bake in the public server so users never have to type a URL:
//
//	go build -ldflags "-X github.com/mateidumitrascu/typing-practice/internal/tui.DefaultServer=https://example.com/typing" ./cmd/tui
var DefaultServer = "http://localhost:8080"

// ServerEnvVar overrides the server for a single run or a whole shell session.
const ServerEnvVar = "TYPEPRACTICE_SERVER"

type Config struct {
	Server    string    `json:"server"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

// ResolveServer picks the server URL from, in order: the --server flag, the
// TYPEPRACTICE_SERVER env var, the saved config, then the compiled-in default.
// The second return value names the source, for display on the setup screen.
func ResolveServer(flagValue string, cfg Config) (server, source string) {
	switch {
	case flagValue != "":
		return NormalizeServer(flagValue), "--server flag"
	case os.Getenv(ServerEnvVar) != "":
		return NormalizeServer(os.Getenv(ServerEnvVar)), ServerEnvVar
	case cfg.Server != "":
		return NormalizeServer(cfg.Server), "saved config"
	default:
		return NormalizeServer(DefaultServer), "default"
	}
}

// NormalizeServer cleans up hand-typed URLs: it adds a scheme when missing
// (https, unless it's clearly a local address) and drops any trailing slash.
func NormalizeServer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		scheme := "https://"
		if host, _, _ := strings.Cut(s, "/"); host == "localhost" ||
			strings.HasPrefix(host, "localhost:") || strings.HasPrefix(host, "127.0.0.1") {
			scheme = "http://"
		}
		s = scheme + s
	}
	return strings.TrimSuffix(s, "/")
}

// ValidServer reports whether s parses as an absolute http(s) URL.
func ValidServer(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "typepractice", "config.json"), nil
}

func LoadConfig() Config {
	var cfg Config
	path, err := configPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	return cfg
}

func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
