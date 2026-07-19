// Package client is the Go API client used by the TUI.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mateidumitrascu/typing-practice/internal/store"
	"github.com/mateidumitrascu/typing-practice/internal/theme"
)

var ErrUnauthorized = errors.New("unauthorized")

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
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
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
