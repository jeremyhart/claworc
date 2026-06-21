package llmgateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gluk-w/claworc/control-plane/internal/config"
)

// writeCreds writes a Claude Code credentials file into dir and points the
// gateway's config at it, restoring the previous value on cleanup.
func writeCreds(t *testing.T, dir string, creds claudeAiOauthCreds) {
	t.Helper()
	body, _ := json.Marshal(claudeCredentialsFile{ClaudeAiOauth: creds})
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), body, 0600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	prev := config.Cfg.ClaudeConfigDir
	config.Cfg.ClaudeConfigDir = dir
	t.Cleanup(func() { config.Cfg.ClaudeConfigDir = prev })
}

func TestAnthropicSubscriptionStatus_Linked(t *testing.T) {
	dir := t.TempDir()
	exp := time.Now().Add(time.Hour).UnixMilli()
	writeCreds(t, dir, claudeAiOauthCreds{
		AccessToken:      "sk-ant-oat01-abc",
		RefreshToken:     "sk-ant-ort01-def",
		ExpiresAt:        exp,
		SubscriptionType: "max",
		Scopes:           []string{"user:inference"},
	})

	st := GetAnthropicSubscriptionStatus()
	if !st.Linked {
		t.Fatal("expected Linked")
	}
	if st.SubscriptionType != "max" || st.ExpiresAt != exp {
		t.Errorf("unexpected status: %+v", st)
	}
}

func TestAnthropicSubscriptionStatus_NotLinked(t *testing.T) {
	dir := t.TempDir()
	prev := config.Cfg.ClaudeConfigDir
	config.Cfg.ClaudeConfigDir = dir
	t.Cleanup(func() { config.Cfg.ClaudeConfigDir = prev })

	if GetAnthropicSubscriptionStatus().Linked {
		t.Error("expected not linked with no credentials file")
	}
}

func TestEnsureFreshAnthropicToken_NotExpired(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, claudeAiOauthCreds{
		AccessToken: "sk-ant-oat01-live",
		ExpiresAt:   time.Now().Add(time.Hour).UnixMilli(),
	})

	tok, err := EnsureFreshAnthropicToken(context.Background())
	if err != nil {
		t.Fatalf("EnsureFreshAnthropicToken: %v", err)
	}
	if tok != "sk-ant-oat01-live" {
		t.Errorf("token = %q", tok)
	}
}

func TestEnsureFreshAnthropicToken_NotLinked(t *testing.T) {
	dir := t.TempDir()
	prev := config.Cfg.ClaudeConfigDir
	config.Cfg.ClaudeConfigDir = dir
	t.Cleanup(func() { config.Cfg.ClaudeConfigDir = prev })

	if _, err := EnsureFreshAnthropicToken(context.Background()); err == nil {
		t.Error("expected error when subscription not linked")
	}
}

func TestEnsureFreshAnthropicToken_ExpiredNoRefreshCmdUsesExisting(t *testing.T) {
	dir := t.TempDir()
	// Expired but still readable; no refresh command configured.
	writeCreds(t, dir, claudeAiOauthCreds{
		AccessToken: "sk-ant-oat01-stale",
		ExpiresAt:   time.Now().Add(time.Hour).UnixMilli(), // future so it counts as valid fallback
	})
	// Force it into the refresh window by backdating expiry just inside the skew.
	writeCreds(t, dir, claudeAiOauthCreds{
		AccessToken: "sk-ant-oat01-stale",
		ExpiresAt:   time.Now().Add(time.Minute).UnixMilli(), // < anthropicRefreshSkew
	})
	prevCmd := config.Cfg.ClaudeRefreshCmd
	config.Cfg.ClaudeRefreshCmd = ""
	t.Cleanup(func() { config.Cfg.ClaudeRefreshCmd = prevCmd })

	// Refresh fails (no command), but the token is not yet past expiry, so the
	// existing token is returned rather than erroring.
	tok, err := EnsureFreshAnthropicToken(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to existing token, got error: %v", err)
	}
	if tok != "sk-ant-oat01-stale" {
		t.Errorf("token = %q", tok)
	}
}

func TestDisconnectAnthropicSubscription(t *testing.T) {
	dir := t.TempDir()
	writeCreds(t, dir, claudeAiOauthCreds{AccessToken: "x", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()})

	if err := DisconnectAnthropicSubscription(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if GetAnthropicSubscriptionStatus().Linked {
		t.Error("expected unlinked after disconnect")
	}
	// Idempotent: disconnecting again is not an error.
	if err := DisconnectAnthropicSubscription(); err != nil {
		t.Errorf("second disconnect: %v", err)
	}
}
