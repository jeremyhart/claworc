// oauth_anthropic.go: Claude subscription (Claude Code OAuth) token resolution
// for the anthropic-oauth api_type.
//
// Unlike openai-codex-responses (which stores OAuth tokens on the LLMProvider
// row and refreshes them itself against auth.openai.com), the Anthropic
// subscription credential lives in a file on the control-plane host, written
// and refreshed by the `claude` CLI (Claude Code) after a one-time
// `claude login`. The gateway only READS that file. When the cached access
// token is near expiry, it shells out to the configured refresh command (the
// `claude` CLI owns the actual OAuth refresh) and re-reads the file — the
// gateway never talks to the OAuth token endpoint directly.
//
// Credentials file (Linux): $CLAWORC_CLAUDE_CONFIG_DIR/.credentials.json
// (default ~/.claude/.credentials.json), shape:
//
//	{ "claudeAiOauth": {
//	    "accessToken": "sk-ant-oat01-...",
//	    "refreshToken": "sk-ant-ort01-...",
//	    "expiresAt": 1730000000000,        // unix ms
//	    "scopes": ["user:inference", "user:profile"],
//	    "subscriptionType": "max"
//	} }

package llmgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gluk-w/claworc/control-plane/internal/config"
)

const (
	// ClaudeOAuthClientID is the public OAuth client id used by the Claude Code
	// CLI. It is the only client id that claude.ai accepts for this flow.
	ClaudeOAuthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// ClaudeOAuthRedirectURI is the registered redirect URI for the manual
	// (paste-URL) login flow. Unlike the Codex localhost URI, this one resolves
	// to a real page — the user copies the full URL from the address bar after
	// being redirected there.
	ClaudeOAuthRedirectURI  = "https://platform.claude.com/oauth/code/callback"
	ClaudeOAuthAuthorizeURL = "https://claude.ai/oauth/authorize"
	ClaudeOAuthTokenURL     = "https://platform.claude.com/v1/oauth/token"
	ClaudeOAuthRolesURL     = "https://api.anthropic.com/api/oauth/claude_cli/roles"
	ClaudeOAuthScope        = "user:inference user:profile org:create_api_key user:sessions:claude_code user:mcp_servers user:file_upload"
)

// anthropicHTTPClient is used for token exchange and roles lookups.
var anthropicHTTPClient = &http.Client{Timeout: 30 * time.Second}

type claudeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// AnthropicOAuthBetaHeader is the anthropic-beta value required to authenticate
// /v1/messages with a Claude subscription OAuth bearer token instead of an
// x-api-key. Confirmed against the Claude OAuth flow.
const AnthropicOAuthBetaHeader = "oauth-2025-04-20"

// anthropicRefreshSkew is how long before expiry we proactively trigger a CLI
// refresh. Any request landing inside this window runs the refresh command.
const anthropicRefreshSkew = 5 * time.Minute

// anthropicOAuthMu serializes credential refreshes — a burst of in-flight
// gateway requests triggers at most one `claude` refresh invocation.
var anthropicOAuthMu sync.Mutex

// claudeAiOauthCreds mirrors the `claudeAiOauth` object in the Claude Code
// credentials file. Only the fields we use are mapped.
type claudeAiOauthCreds struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // unix ms
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
}

type claudeCredentialsFile struct {
	ClaudeAiOauth claudeAiOauthCreds `json:"claudeAiOauth"`
}

// claudeConfigDir returns the directory holding the Claude Code credentials
// file: CLAWORC_CLAUDE_CONFIG_DIR if set, else ~/.claude.
func claudeConfigDir() string {
	if d := strings.TrimSpace(config.Cfg.ClaudeConfigDir); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Fall back to the conventional container home.
		home = "/root"
	}
	return filepath.Join(home, ".claude")
}

// claudeCredentialsPath returns the absolute path to the credentials file.
func claudeCredentialsPath() string {
	return filepath.Join(claudeConfigDir(), ".credentials.json")
}

// readClaudeCredentials reads and parses the Claude Code credentials file.
func readClaudeCredentials() (*claudeAiOauthCreds, error) {
	path := claudeCredentialsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Claude subscription not linked (no credentials at %s); run `claude login` on the orchestrator", path)
		}
		return nil, fmt.Errorf("read claude credentials: %w", err)
	}
	var f claudeCredentialsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse claude credentials: %w", err)
	}
	if f.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("claude credentials at %s have no accessToken; re-run `claude login`", path)
	}
	return &f.ClaudeAiOauth, nil
}

// EnsureFreshAnthropicToken returns a non-expired Claude subscription access
// token, triggering a CLI refresh (CLAWORC_CLAUDE_REFRESH_CMD) when the cached
// token is within the refresh window. Returns an error the caller should
// surface as a 401 with a hint to re-link the subscription.
func EnsureFreshAnthropicToken(ctx context.Context) (string, error) {
	anthropicOAuthMu.Lock()
	defer anthropicOAuthMu.Unlock()

	creds, err := readClaudeCredentials()
	if err != nil {
		return "", err
	}

	now := time.Now().UnixMilli()
	if creds.ExpiresAt == 0 || creds.ExpiresAt-now > anthropicRefreshSkew.Milliseconds() {
		return creds.AccessToken, nil
	}

	// Token is near or past expiry — let the claude CLI rotate it, then re-read.
	if refreshErr := runClaudeRefresh(ctx); refreshErr != nil {
		// If the (possibly stale) token is still valid, use it rather than failing.
		if creds.ExpiresAt > now {
			return creds.AccessToken, nil
		}
		return "", fmt.Errorf("claude token expired and refresh failed: %w", refreshErr)
	}

	refreshed, err := readClaudeCredentials()
	if err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

// runClaudeRefresh executes the configured refresh command. The `claude` CLI
// owns the OAuth refresh against Anthropic's token endpoint; the gateway only
// invokes it and re-reads the file. When CLAWORC_CLAUDE_REFRESH_CMD is unset
// the refresh is a no-op (rely on an external keep-alive / scheduled job to
// keep the credentials file fresh).
func runClaudeRefresh(ctx context.Context) error {
	cmdline := strings.TrimSpace(config.Cfg.ClaudeRefreshCmd)
	if cmdline == "" {
		return fmt.Errorf("no CLAWORC_CLAUDE_REFRESH_CMD configured")
	}
	parts := strings.Fields(cmdline)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	// Ensure the CLI reads the same config dir the gateway reads from.
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+claudeConfigDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		snippet := strings.TrimSpace(string(out))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("refresh command failed: %v: %s", err, snippet)
	}
	return nil
}

// AnthropicSubscriptionStatus describes the linked Claude subscription for the
// settings UI. Linked is false when no usable credentials file is present.
type AnthropicSubscriptionStatus struct {
	Linked           bool     `json:"linked"`
	SubscriptionType string   `json:"subscription_type,omitempty"`
	ExpiresAt        int64    `json:"expires_at,omitempty"` // unix ms
	Scopes           []string `json:"scopes,omitempty"`
	CredentialsPath  string   `json:"credentials_path"`
}

// GetAnthropicSubscriptionStatus reports whether a Claude subscription is
// linked on the orchestrator, for display in the UI. It never returns tokens.
func GetAnthropicSubscriptionStatus() AnthropicSubscriptionStatus {
	status := AnthropicSubscriptionStatus{CredentialsPath: claudeCredentialsPath()}
	creds, err := readClaudeCredentials()
	if err != nil {
		return status
	}
	status.Linked = true
	status.SubscriptionType = creds.SubscriptionType
	status.ExpiresAt = creds.ExpiresAt
	status.Scopes = creds.Scopes
	return status
}

// DisconnectAnthropicSubscription removes the credentials file, unlinking the
// shared subscription. The `claude` CLI will need `claude login` again to relink.
func DisconnectAnthropicSubscription() error {
	anthropicOAuthMu.Lock()
	defer anthropicOAuthMu.Unlock()
	path := claudeCredentialsPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials: %w", err)
	}
	return nil
}

// exchangeAnthropicAuthCode exchanges an OAuth authorization code for tokens
// using the Claude Code PKCE flow. Called by LinkAnthropicSubscription.
func exchangeAnthropicAuthCode(ctx context.Context, code, codeVerifier string) (*claudeTokenResponse, error) {
	payload, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClaudeOAuthClientID,
		"code":          code,
		"code_verifier": codeVerifier,
		"redirect_uri":  ClaudeOAuthRedirectURI,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ClaudeOAuthTokenURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := anthropicHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, snippet)
	}
	var out claudeTokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty access_token")
	}
	return &out, nil
}

// fetchAnthropicSubscriptionType calls the Claude CLI roles endpoint to
// retrieve the user's subscription plan name (e.g. "max", "pro"). Returns ""
// on any error — the subscription will still work; only the display label is missing.
func fetchAnthropicSubscriptionType(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ClaudeOAuthRolesURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", AnthropicOAuthBetaHeader)

	resp, err := anthropicHTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result struct {
		SubscriptionType string `json:"subscription_type"`
		Plan             string `json:"plan"`
		Role             string `json:"role"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	if result.SubscriptionType != "" {
		return result.SubscriptionType
	}
	if result.Plan != "" {
		return result.Plan
	}
	return result.Role
}

// writeClaudeCredentials atomically writes the credentials file that the
// gateway and the `claude` CLI both read from.
func writeClaudeCredentials(creds claudeAiOauthCreds) error {
	path := claudeCredentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	file := claudeCredentialsFile{ClaudeAiOauth: creds}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install credentials: %w", err)
	}
	return nil
}

// LinkAnthropicSubscription exchanges an OAuth authorization code (obtained
// via the browser PKCE flow) for tokens and writes them to the credentials
// file, linking the shared Claude subscription without requiring a terminal.
func LinkAnthropicSubscription(ctx context.Context, codeVerifier, redirectURL string) error {
	anthropicOAuthMu.Lock()
	defer anthropicOAuthMu.Unlock()

	code, _, err := ExtractCodeAndState(redirectURL)
	if err != nil {
		return err
	}
	if code == "" {
		return fmt.Errorf("redirect URL does not contain an auth code")
	}

	tok, err := exchangeAnthropicAuthCode(ctx, code, codeVerifier)
	if err != nil {
		return err
	}

	scopes := strings.Fields(tok.Scope)
	if len(scopes) == 0 {
		scopes = strings.Fields(ClaudeOAuthScope)
	}

	subType := fetchAnthropicSubscriptionType(ctx, tok.AccessToken)

	creds := claudeAiOauthCreds{
		AccessToken:      tok.AccessToken,
		RefreshToken:     tok.RefreshToken,
		ExpiresAt:        time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UnixMilli(),
		Scopes:           scopes,
		SubscriptionType: subType,
	}
	return writeClaudeCredentials(creds)
}
