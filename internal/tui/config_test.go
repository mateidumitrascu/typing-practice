package tui

import "testing"

func TestNormalizeServer(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"  https://x.dev/typing/  ": "https://x.dev/typing",
		"example.com/typing":        "https://example.com/typing",
		"localhost:8080":            "http://localhost:8080",
		"127.0.0.1:8080":            "http://127.0.0.1:8080",
		"http://example.com":        "http://example.com",
	}
	for in, want := range cases {
		if got := NormalizeServer(in); got != want {
			t.Errorf("NormalizeServer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidServer(t *testing.T) {
	for _, s := range []string{"http://localhost:8080", "https://x.dev/typing"} {
		if !ValidServer(s) {
			t.Errorf("ValidServer(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "notaurl", "ftp://x.dev", "https://"} {
		if ValidServer(s) {
			t.Errorf("ValidServer(%q) = true, want false", s)
		}
	}
}

func TestResolveServerPrecedence(t *testing.T) {
	cfg := Config{Server: "https://saved.dev"}

	t.Run("flag wins", func(t *testing.T) {
		t.Setenv(ServerEnvVar, "https://env.dev")
		got, src := ResolveServer("https://flag.dev", cfg)
		if got != "https://flag.dev" || src != "--server flag" {
			t.Errorf("got %q from %q", got, src)
		}
	})

	t.Run("env beats config", func(t *testing.T) {
		t.Setenv(ServerEnvVar, "https://env.dev")
		got, src := ResolveServer("", cfg)
		if got != "https://env.dev" || src != ServerEnvVar {
			t.Errorf("got %q from %q", got, src)
		}
	})

	t.Run("config beats default", func(t *testing.T) {
		t.Setenv(ServerEnvVar, "")
		got, src := ResolveServer("", cfg)
		if got != "https://saved.dev" || src != "saved config" {
			t.Errorf("got %q from %q", got, src)
		}
	})

	t.Run("falls back to default", func(t *testing.T) {
		t.Setenv(ServerEnvVar, "")
		got, src := ResolveServer("", Config{})
		if got != NormalizeServer(DefaultServer) || src != "default" {
			t.Errorf("got %q from %q", got, src)
		}
	})
}
