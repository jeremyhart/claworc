package llmgateway

import (
	"encoding/json"
	"strings"
)

// claudeCodeIdentity is the system-prompt identity that Anthropic requires as
// the FIRST system block when authenticating /v1/messages with a Claude
// subscription OAuth token. Without it, subscription requests are rejected.
const claudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."

// rewriteAnthropicOAuthRequestBody ensures the request's `system` field begins
// with the Claude Code identity block. OpenClaw's own system prompt is
// preserved and follows the injected identity. Best-effort: on any JSON parse
// error or unexpected shape the original body is returned unchanged.
//
// The Anthropic Messages API accepts `system` as either a plain string or an
// array of content blocks; this handles both, plus the absent case.
func rewriteAnthropicOAuthRequestBody(body []byte) []byte {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil || doc == nil {
		return body
	}

	identityBlock := map[string]any{"type": "text", "text": claudeCodeIdentity}

	switch sys := doc["system"].(type) {
	case nil:
		doc["system"] = []any{identityBlock}
	case string:
		if strings.HasPrefix(strings.TrimSpace(sys), claudeCodeIdentity) {
			return body
		}
		doc["system"] = []any{identityBlock, map[string]any{"type": "text", "text": sys}}
	case []any:
		if anthropicSystemHasIdentity(sys) {
			return body
		}
		doc["system"] = append([]any{identityBlock}, sys...)
	default:
		// Unknown shape — leave it alone.
		return body
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return out
}

// anthropicSystemHasIdentity reports whether the first text block of a
// system-blocks array already carries the Claude Code identity.
func anthropicSystemHasIdentity(blocks []any) bool {
	if len(blocks) == 0 {
		return false
	}
	first, ok := blocks[0].(map[string]any)
	if !ok {
		return false
	}
	text, _ := first["text"].(string)
	return strings.HasPrefix(strings.TrimSpace(text), claudeCodeIdentity)
}
