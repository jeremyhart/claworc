// Package client is a thin HTTP client for the Claworc control-plane REST API
// that implements the tools.Doer interface.
//
// Authentication priority:
//  1. Bearer token (CLAWORC_TOKEN) — sent as "Authorization: Bearer <token>" on
//     every request; no session state needed.
//  2. Username/password (CLAWORC_USERNAME + CLAWORC_PASSWORD) — exchanges
//     credentials for a session cookie via POST /api/v1/auth/login and
//     re-authenticates automatically on HTTP 401.
//
// API() prefixes the path with /api/v1; Raw() uses the path verbatim.
package client

import (
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

	"github.com/gluk-w/claworc/mcp-server/internal/tools"
)

// Config holds the connection settings, populated from environment variables.
type Config struct {
	BaseURL  string        // e.g. http://localhost:8000
	Token    string        // bearer token (preferred auth)
	Username string        // fallback: username/password login
	Password string        // fallback: username/password login
	Insecure bool          // skip TLS verification (self-signed certs)
	Timeout  time.Duration // per-request timeout
}

// Client is a stateful API client implementing tools.Doer. It is safe for
// concurrent use; the mutex keeps the login/retry dance race-free.
type Client struct {
	cfg      Config
	http     *http.Client
	mu       sync.Mutex
	loggedIn bool
}

// Compile-time assertion that *Client satisfies tools.Doer.
var _ tools.Doer = (*Client)(nil)

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
		//nolint:gosec // intentional: controlled by CLAWORC_INSECURE env var
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

// hasToken reports whether a bearer token is configured (preferred auth path).
func (c *Client) hasToken() bool {
	return c.cfg.Token != ""
}

// hasCredentials reports whether a username/password pair was supplied.
func (c *Client) hasCredentials() bool {
	return c.cfg.Username != "" && c.cfg.Password != ""
}

// login exchanges the configured credentials for a session cookie.
func (c *Client) login(ctx context.Context) error {
	if !c.hasCredentials() {
		// Assume auth disabled on the control plane.
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
// startup rather than on the first tool call. Skipped when using bearer auth.
func (c *Client) EnsureLogin(ctx context.Context) error {
	if c.hasToken() {
		return nil // token auth — no pre-flight login needed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	return c.login(ctx)
}

// API implements tools.Doer: performs a request against /api/v1 + path.
func (c *Client) API(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	return c.do(ctx, method, "/api/v1"+ensureLeadingSlash(path), query, body)
}

// Raw implements tools.Doer: performs a request against an arbitrary path.
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	return c.do(ctx, method, ensureLeadingSlash(path), query, body)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	// Bearer token path: no session state, no mutex needed for auth.
	if c.hasToken() {
		return c.send(ctx, method, path, query, body)
	}

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

	// Session expired: re-login once and retry.
	if resp.Status == http.StatusUnauthorized && c.hasCredentials() {
		c.loggedIn = false
		if err := c.login(ctx); err != nil {
			return nil, err
		}
		return c.send(ctx, method, path, query, body)
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
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

	// Attach bearer token when configured.
	if c.hasToken() {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", u, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return &tools.Result{Status: resp.StatusCode, Body: data}, nil
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
