// Package client is the Go API client used by the TUI.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mateidumitrascu/typing-practice/internal/store"
	"github.com/mateidumitrascu/typing-practice/internal/theme"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	// ErrUnavailable means the server could not be reached, or a proxy in
	// front of it (Cloudflare) could not reach the server. Callers can offer
	// a retry rather than treating it as a bug in the request.
	ErrUnavailable = errors.New("server unavailable")
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(base, token string) *Client {
	return &Client{
		base:  strings.TrimSuffix(base, "/"),
		token: token,
		http:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SetToken(token string) { c.token = token }

func (c *Client) do(method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return transportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		return responseError(resp)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// transportError turns Go's verbose network errors into something worth
// showing a user mid-typing-test.
func transportError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		switch {
		case urlErr.Timeout():
			return fmt.Errorf("%w: timed out", ErrUnavailable)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("%w: connection closed", ErrUnavailable)
		}
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return fmt.Errorf("%w: cannot resolve %s", ErrUnavailable, dnsErr.Name)
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return fmt.Errorf("%w: connection refused", ErrUnavailable)
		}
		return fmt.Errorf("%w: %v", ErrUnavailable, urlErr.Err)
	}
	return err
}

// responseError builds an error from a 4xx/5xx response. Gateway statuses get
// a plain-language message: those come from the proxy, not the app, and their
// bodies are HTML error pages rather than our JSON.
func responseError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Errorf("%w: the server is down or restarting (%d)", ErrUnavailable, resp.StatusCode)
	case http.StatusTooManyRequests:
		return errors.New("too many attempts, wait a minute and try again")
	}
	// Only trust a JSON body to hold our own error message; anything else
	// (an HTML proxy page, say) would decode into noise.
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		var e struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: server error (%d)", ErrUnavailable, resp.StatusCode)
	}
	return fmt.Errorf("server returned %s", resp.Status)
}

func (c *Client) Login(username, password string) (token string, expiresAt time.Time, err error) {
	var resp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	err = c.do("POST", "/api/auth/login", map[string]string{
		"username": username, "password": password,
	}, &resp)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			err = errors.New("invalid username or password")
		}
		return "", time.Time{}, err
	}
	c.token = resp.Token
	return resp.Token, resp.ExpiresAt, nil
}

func (c *Client) Logout() error {
	return c.do("POST", "/api/auth/logout", nil, nil)
}

func (c *Client) Words(mode string, letter rune) ([]string, error) {
	path := "/api/words?mode=" + mode
	if mode == "letter" {
		path += "&letter=" + string(letter)
	}
	var resp struct {
		Words []string `json:"words"`
	}
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Words, nil
}

func (c *Client) Themes() (themes []theme.Theme, defaultName string, err error) {
	var resp struct {
		Themes  []theme.Theme `json:"themes"`
		Default string        `json:"default"`
	}
	if err := c.do("GET", "/api/themes", nil, &resp); err != nil {
		return nil, "", err
	}
	return resp.Themes, resp.Default, nil
}

func (c *Client) SubmitResult(r store.Result) error {
	return c.do("POST", "/api/results", r, nil)
}

func (c *Client) Stats() (store.Stats, error) {
	var st store.Stats
	err := c.do("GET", "/api/stats", nil, &st)
	return st, err
}

func (c *Client) Theme() (string, error) {
	var resp struct {
		Theme string `json:"theme"`
	}
	if err := c.do("GET", "/api/settings/tui", nil, &resp); err != nil {
		return "", err
	}
	return resp.Theme, nil
}

func (c *Client) SetTheme(name string) error {
	return c.do("PUT", "/api/settings/tui", map[string]string{"theme": name}, nil)
}
