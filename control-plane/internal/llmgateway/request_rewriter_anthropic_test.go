package llmgateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// systemBlocks unmarshals the rewritten body and returns its system field as a
// slice of blocks, failing the test if it isn't an array.
func systemBlocks(t *testing.T, body []byte) []any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal rewritten body: %v", err)
	}
	blocks, ok := doc["system"].([]any)
	if !ok {
		t.Fatalf("system is not an array: %T", doc["system"])
	}
	return blocks
}

func firstBlockText(t *testing.T, blocks []any) string {
	t.Helper()
	if len(blocks) == 0 {
		t.Fatal("no system blocks")
	}
	b, ok := blocks[0].(map[string]any)
	if !ok {
		t.Fatalf("first block not an object: %T", blocks[0])
	}
	text, _ := b["text"].(string)
	return text
}

func TestRewriteAnthropicOAuth_StringSystem(t *testing.T) {
	in := []byte(`{"model":"claude-x","system":"You are a helpful agent.","messages":[]}`)
	out := rewriteAnthropicOAuthRequestBody(in)
	blocks := systemBlocks(t, out)
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(blocks))
	}
	if got := firstBlockText(t, blocks); got != claudeCodeIdentity {
		t.Errorf("first block = %q, want identity", got)
	}
	// Original system preserved as the second block.
	second, _ := blocks[1].(map[string]any)
	if second["text"] != "You are a helpful agent." {
		t.Errorf("second block = %v, want original system", second["text"])
	}
}

func TestRewriteAnthropicOAuth_AbsentSystem(t *testing.T) {
	in := []byte(`{"model":"claude-x","messages":[]}`)
	out := rewriteAnthropicOAuthRequestBody(in)
	blocks := systemBlocks(t, out)
	if len(blocks) != 1 || firstBlockText(t, blocks) != claudeCodeIdentity {
		t.Fatalf("absent system should yield a single identity block, got %v", blocks)
	}
}

func TestRewriteAnthropicOAuth_ArraySystemPrepends(t *testing.T) {
	in := []byte(`{"system":[{"type":"text","text":"do the thing"}],"messages":[]}`)
	out := rewriteAnthropicOAuthRequestBody(in)
	blocks := systemBlocks(t, out)
	if len(blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(blocks))
	}
	if firstBlockText(t, blocks) != claudeCodeIdentity {
		t.Errorf("identity not prepended")
	}
}

func TestRewriteAnthropicOAuth_AlreadyPresentNoOp(t *testing.T) {
	in := []byte(`{"system":[{"type":"text","text":"` + claudeCodeIdentity + `"},{"type":"text","text":"x"}],"messages":[]}`)
	out := rewriteAnthropicOAuthRequestBody(in)
	blocks := systemBlocks(t, out)
	if len(blocks) != 2 {
		t.Fatalf("identity already present should not add a block, got %d", len(blocks))
	}
}

func TestRewriteAnthropicOAuth_InvalidJSONUnchanged(t *testing.T) {
	in := []byte(`not json`)
	if string(rewriteAnthropicOAuthRequestBody(in)) != string(in) {
		t.Error("invalid JSON should be returned unchanged")
	}
}

func TestAnthropicOAuthSetAuthHeader(t *testing.T) {
	t.Run("bearer + beta, no x-api-key", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
		anthropicOAuth{}.SetAuthHeader(req, AuthMaterial{OAuthAccess: "tok123"})
		if got := req.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("Authorization = %q", got)
		}
		if got := req.Header.Get("anthropic-beta"); got != AnthropicOAuthBetaHeader {
			t.Errorf("anthropic-beta = %q", got)
		}
		if req.Header.Get("x-api-key") != "" {
			t.Error("x-api-key should not be set")
		}
	})

	t.Run("merges existing anthropic-beta", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
		req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
		anthropicOAuth{}.SetAuthHeader(req, AuthMaterial{OAuthAccess: "tok"})
		got := req.Header.Get("anthropic-beta")
		if !strings.Contains(got, "prompt-caching-2024-07-31") || !strings.Contains(got, AnthropicOAuthBetaHeader) {
			t.Errorf("anthropic-beta = %q, want both flags", got)
		}
	})
}
