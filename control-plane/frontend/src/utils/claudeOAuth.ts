// PKCE helpers and authorize-URL builder for the anthropic-oauth
// (Claude subscription) login flow. Constants mirror those in
// control-plane/internal/llmgateway/oauth_anthropic.go — keep them in sync.
//
// Unlike the Codex flow (localhost redirect that never loads), the Claude
// redirect lands on a real page at platform.claude.com. The user copies the
// full URL from the address bar after authorisation and pastes it back.
// The verifier and state are held in React component state only — closing the
// modal mid-flow leaves nothing behind.

export const CLAUDE_OAUTH = {
  client_id: "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
  redirect_uri: "https://platform.claude.com/oauth/code/callback",
  authorize_url: "https://claude.ai/oauth/authorize",
  scope: "user:inference user:profile org:create_api_key user:sessions:claude_code user:mcp_servers user:file_upload",
} as const;

export function buildClaudeAuthorizeURL(state: string, challenge: string): string {
  const params = new URLSearchParams({
    response_type: "code",
    client_id: CLAUDE_OAUTH.client_id,
    redirect_uri: CLAUDE_OAUTH.redirect_uri,
    scope: CLAUDE_OAUTH.scope,
    state,
    code_challenge: challenge,
    code_challenge_method: "S256",
  });
  return `${CLAUDE_OAUTH.authorize_url}?${params.toString()}`;
}
