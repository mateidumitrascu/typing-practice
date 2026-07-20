package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve runs a client request against a handler and returns the error.
func serve(t *testing.T, h http.HandlerFunc) error {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	_, err := New(srv.URL, "tok").Words("speed", 0)
	return err
}

// A Cloudflare 502 returns an HTML page, not our JSON error shape.
func TestGatewayErrorsAreFriendly(t *testing.T) {
	for _, code := range []int{502, 503, 504} {
		err := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(code)
			w.Write([]byte("<html><title>502 Bad Gateway</title><body>cloudflare</body></html>"))
		})
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("status %d: got %v, want ErrUnavailable", code, err)
		}
		if strings.Contains(err.Error(), "html") || strings.Contains(err.Error(), "cloudflare") {
			t.Errorf("status %d: raw page leaked into message: %v", code, err)
		}
	}
}

func TestJSONErrorMessagePreserved(t *testing.T) {
	err := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"mode must be speed, free or letter"}`))
	})
	if err == nil || err.Error() != "mode must be speed, free or letter" {
		t.Errorf("got %v, want the server's own message", err)
	}
}

func TestUnauthorized(t *testing.T) {
	err := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}

func TestRateLimitMessage(t *testing.T) {
	err := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if err == nil || !strings.Contains(err.Error(), "wait a minute") {
		t.Errorf("got %v, want a wait-and-retry message", err)
	}
}

func TestUnreachableServer(t *testing.T) {
	// Port 1 on localhost refuses connections.
	_, err := New("http://127.0.0.1:1", "").Words("speed", 0)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v, want ErrUnavailable", err)
	}
}

func TestBadHostname(t *testing.T) {
	_, err := New("http://nonexistent.invalid", "").Words("speed", 0)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v, want ErrUnavailable", err)
	}
}
