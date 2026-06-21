// Package client is a thin HTTP client for the Claworc control-plane REST API.
//
// It maintains a session cookie obtained by logging in with a username and
// password, transparently re-authenticating when the control plane reports the
// session has expired (HTTP 401). All control-plane endpoints live under
// /api/v1; the API helper prefixes that automatically, while Raw talks to an
// arbitrary path for the generic escape-hatch tool.
package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Config holds the connection settings, populated from environment variables.
type Config struct {
	BaseURL  string // e.g. http://localhost:8000
	Username string
	Password string
	Insecure bool          // skip TLS verification (self-signed certs)
	Timeout  time.Duration // per-request timeout
}

// Client is a stateful API client. It is safe for concurrent use; the MCP
// server invokes tool handlers serially in practice, but the mutex keeps the
// login/retry dance race-free regardless.
type Client struct {
	cfg      Config
	http     *http.Client
	mu       sync.Mutex
	loggedIn bool
}

// Response is the normalized result of an API call.
type Response struct {
	Status int
	Body   []byte
}

// New constructs a Client from cfg. It returns an error only when the
// configuration is unusable (e.g. a cookie jar cannot be created).
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8000"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		cfg: cfg,
		http: &http.Client{
			Jar:       jar,
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
	}, nil
}

// hasCredentials reports whether a username/password pair was supplied. When
// the control plane runs with CLAWORC_AUTH_DISABLED=true no login is needed.
func (c *Client) hasCredentials() bool {
	return c.cfg.Username != "" && c.cfg.Password != ""
}

// login exchanges the configured credentials for a session cookie, which the
// cookie jar then attaches to subsequent requests automatically.
func (c *Client) login(ctx context.Context) error {
	if !c.hasCredentials() {
		// Nothing to do — assume the control plane has auth disabled.
		c.loggedIn = true
		return nil
	}

	payload, _ := json.Marshal(map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	c.loggedIn = true
	return nil
}

// EnsureLogin performs an initial login so configuration problems surface at
// startup rather than on the first tool call.
func (c *Client) EnsureLogin(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	return c.login(ctx)
}

// API performs a request against /api/v1 + path. body is JSON-encoded when
// non-nil. It re-authenticates once on a 401 response.
func (c *Client) API(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	return c.do(ctx, method, "/api/v1"+ensureLeadingSlash(path), query, body)
}

// Raw performs a request against an arbitrary path (escape hatch). The path is
// appended to the base URL verbatim, so callers pass e.g. "/api/v1/instances"
// or "/health".
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	return c.do(ctx, method, ensureLeadingSlash(path), query, body)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}

	resp, err := c.send(ctx, method, path, query, body)
	if err != nil {
		return nil, err
	}

	// Session expired: re-login once and retry the request.
	if resp.Status == http.StatusUnauthorized && c.hasCredentials() {
		c.loggedIn = false
		if err := c.login(ctx); err != nil {
			return nil, err
		}
		return c.send(ctx, method, path, query, body)
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	u := c.cfg.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		raw, ok := body.(json.RawMessage)
		if ok {
			reader = bytes.NewReader(raw)
		} else {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("encode request body: %w", err)
			}
			reader = bytes.NewReader(encoded)
		}
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), u, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", u, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return &Response{Status: resp.StatusCode, Body: data}, nil
}

// ParseSSE extracts the payloads of "data:" lines from a Server-Sent Events
// body, joining them with newlines. The control plane streams instance logs
// this way; with follow=false the stream terminates after the requested tail.
func ParseSSE(body []byte) string {
	var b strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	return b.String()
}

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}
