package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"golang.org/x/crypto/bcrypt"

	"github.com/mateidumitrascu/typepractice/internal/config"
	"github.com/mateidumitrascu/typepractice/internal/store"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err := st.CreateUser(t.Context(), "matei", string(hash)); err != nil {
		t.Fatal(err)
	}
	return New(st, config.Default(), fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}})
}

func do(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func login(t *testing.T, h http.Handler) string {
	t.Helper()
	w := do(t, h, "POST", "/api/auth/login", "", map[string]string{
		"username": "matei", "password": "secret123",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	var resp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token
}

func TestLoginRejectsBadPassword(t *testing.T) {
	h := newTestServer(t)
	w := do(t, h, "POST", "/api/auth/login", "", map[string]string{
		"username": "matei", "password": "wrong",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	h := newTestServer(t)
	last := 0
	for range 10 {
		w := do(t, h, "POST", "/api/auth/login", "", map[string]string{"username": "x", "password": "y"})
		last = w.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("got %d after 10 attempts, want 429", last)
	}
}

func TestAuthRequired(t *testing.T) {
	h := newTestServer(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/results"},
		{"GET", "/api/stats"},
		{"POST", "/api/results"},
		{"GET", "/api/settings/tui"},
	} {
		if w := do(t, h, tc.method, tc.path, "", nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: got %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}

func TestWordsPublic(t *testing.T) {
	h := newTestServer(t)
	for _, path := range []string{"/api/words", "/api/words?mode=letter&letter=q", "/api/themes"} {
		w := do(t, h, "GET", path, "", nil)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: got %d, want 200", path, w.Code)
		}
	}
	w := do(t, h, "GET", "/api/words?mode=letter&letter=qq", "", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid letter: got %d, want 400", w.Code)
	}
}

func TestResultFlow(t *testing.T) {
	h := newTestServer(t)
	token := login(t, h)

	w := do(t, h, "POST", "/api/results", token, map[string]any{
		"mode": "speed", "word_count": 40, "duration_ms": 60000,
		"chars_typed": 250, "errors": 5, "accuracy": 97.5,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create result: %d %s", w.Code, w.Body)
	}

	w = do(t, h, "GET", "/api/stats", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", w.Code, w.Body)
	}
	var st store.Stats
	json.Unmarshal(w.Body.Bytes(), &st)
	if st.TotalTests != 1 || st.BestWPM != 50 || st.BestNetWPM != 45 {
		t.Errorf("stats = %+v, want 1 test, best 50/45 (server recomputes wpm)", st)
	}

	w = do(t, h, "GET", "/api/results", token, nil)
	var lr struct {
		Results []store.Result `json:"results"`
	}
	json.Unmarshal(w.Body.Bytes(), &lr)
	if len(lr.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(lr.Results))
	}
}

func TestSettingsFlow(t *testing.T) {
	h := newTestServer(t)
	token := login(t, h)

	w := do(t, h, "GET", "/api/settings/tui", token, nil)
	if body := w.Body.String(); w.Code != http.StatusOK || !bytes.Contains([]byte(body), []byte("carbon")) {
		t.Fatalf("default setting: %d %s", w.Code, body)
	}
	if w = do(t, h, "PUT", "/api/settings/tui", token, map[string]string{"theme": "fjord"}); w.Code != http.StatusNoContent {
		t.Fatalf("put setting: %d %s", w.Code, w.Body)
	}
	if w = do(t, h, "PUT", "/api/settings/tui", token, map[string]string{"theme": "nope"}); w.Code != http.StatusBadRequest {
		t.Errorf("unknown theme: got %d, want 400", w.Code)
	}
	w = do(t, h, "GET", "/api/settings/tui", token, nil)
	if !bytes.Contains(w.Body.Bytes(), []byte("fjord")) {
		t.Errorf("setting not persisted: %s", w.Body)
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	h := newTestServer(t)
	token := login(t, h)
	if w := do(t, h, "POST", "/api/auth/logout", token, nil); w.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", w.Code)
	}
	if w := do(t, h, "GET", "/api/stats", token, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("token still valid after logout: %d", w.Code)
	}
}

func TestBasePathMount(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfg := config.Default()
	cfg.BasePath = "/typing"
	h := New(st, cfg, fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}})

	if w := do(t, h, "GET", "/typing/api/words", "", nil); w.Code != http.StatusOK {
		t.Errorf("GET /typing/api/words: got %d, want 200", w.Code)
	}
	if w := do(t, h, "GET", "/api/words", "", nil); w.Code == http.StatusOK {
		t.Error("unprefixed path should not be served when base path is set")
	}
	w := do(t, h, "GET", "/typing", "", nil)
	if w.Code != http.StatusMovedPermanently {
		t.Errorf("GET /typing: got %d, want 301 redirect", w.Code)
	}
}
