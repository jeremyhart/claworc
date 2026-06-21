# MCP Server — Web-Managed Claworc

Status: **Plan / spec** (not yet implemented). This document is the agreed design
for making a Claworc deployment fully manageable by an LLM through the Model
Context Protocol (MCP), connectable from **Claude Code on the web** and **local
Claude Code / desktop**.

This branch starts clean from `main`; the implementation described here is built
fresh on top of it.

## Background

A throwaway prototype (on branch `claude/mcp-server-build-j55lpc`) shipped a
standalone **stdio** MCP server that wrapped the control-plane REST API and
authenticated with a username/password session cookie. That works for *local*
MCP clients that can spawn a child process, but the web clients cannot use it:
Claude Code on the web and the claude.ai app only connect to **remote MCP servers
over Streamable HTTP at a public HTTPS URL**. This plan delivers that path
properly, embedded in the control plane, and is what we implement on this branch.

## Goals & decisions

- **Transport:** Streamable HTTP, served at `https://<control-plane>/mcp`. Works
  for Claude Code on the web and local Claude Code alike.
- **Hosting:** **embedded in the control plane** — one process, one public URL,
  in-process tool execution (no second network hop, no stored service creds).
- **Auth:** bearer **API tokens** (`Authorization: Bearer claworc_pat_…`). OAuth
  is intentionally out of scope because claude.ai custom connectors are not a
  target; Claude Code (web and local) accepts a bearer header.
- **Token model:** user-scoped, self-service, managed on the Account page
  alongside passkeys. A token inherits its user's role, so all existing
  per-route authorization is reused unchanged.

Non-goals: claude.ai custom connectors (would require OAuth 2.1 + PKCE +
discovery endpoints); machine-to-machine grants.

## Architecture

```
Claude Code (web/local) ──HTTPS Streamable HTTP──▶ /mcp  (control plane)
   Authorization: Bearer claworc_pat_…                │ per-session MCP server
                                                       │ captures caller's auth
                                                       ▼
                              in-process Doer → chi router.ServeHTTP → existing handlers
                                              (RequireAuth / RequireAdmin / team checks all run)
```

### A new `mcp-server` module, transport-agnostic tools

Implementation introduces a Go module at `mcp-server/` (light deps: only the MCP
SDK + an HTTP client) whose tool definitions target a small interface rather than
a concrete transport:

```go
type Doer interface {
    API(ctx context.Context, method, path string, q url.Values, body any) (status int, b []byte, err error)
    Raw(ctx context.Context, method, path string, q url.Values, body any) (status int, b []byte, err error)
}

func Register(s *mcp.Server, d Doer) // one tool set, any transport
```

Two front-ends reuse the identical definitions:

- **Standalone stdio binary** (`mcp-server/`) → `Doer` backed by an HTTP client
  for local use / deployments where embedding isn't wanted.
- **Embedded server** (control plane) → `Doer` backed by in-process
  `router.ServeHTTP` (no socket), replaying the caller's `Authorization` header
  so the full middleware/authorization chain runs exactly as for external calls.

The control plane imports the `mcp-server` module with a
`replace github.com/gluk-w/claworc/mcp-server => ../mcp-server` directive for
monorepo builds.

### Per-session auth capture

`mcp.NewStreamableHTTPHandler(getServer func(*http.Request) *mcp.Server, …)` is
invoked per HTTP session. The factory captures the inbound request's auth header,
builds a fresh `*mcp.Server`, and registers the tool set bound to an in-process
`Doer` that replays that auth. Because the replayed request re-runs `RequireAuth`
with the same bearer token, authorization is identical to an external API call —
there is no privilege bypass.

## Work breakdown

### Phase 1 — API token auth (backend)

| File | Change |
|---|---|
| `internal/database/models/models.go` | New `APIToken{ ID, UserID (index), Name, TokenHash (unique index), Prefix, LastUsedAt *time.Time, ExpiresAt *time.Time, CreatedAt }` |
| `internal/database/models.go` | Add `APIToken = models.APIToken` type alias |
| `internal/database/migrations/migration_00001_baseline.go` | Add `&models.APIToken{}` to `AutoMigrateAll` (additive table → no migration file) |
| `internal/database/database.go` | `CreateAPIToken`, `ListAPITokensByUser`, `GetAPITokenByHash`, `DeleteAPIToken(id, userID)`, `TouchAPITokenLastUsed` |
| `internal/auth/auth.go` | `GenerateAPIToken() (plain, hash, prefix string)` — `claworc_pat_` + 32 random bytes (hex); store only `sha256(plain)` |
| `internal/middleware/auth.go` | In `RequireAuth`: if `Authorization: Bearer claworc_pat_…` present → hash → `GetAPITokenByHash` → load user → check expiry → `TouchAPITokenLastUsed` (throttled). Otherwise fall back to the session cookie. Same context shape, so all downstream guards are unchanged. |
| `internal/handlers/tokens.go` (new) | `ListAPITokens`, `CreateAPIToken` (returns the full secret **once**), `DeleteAPIToken` — mirror the WebAuthn handlers; user-scoped via `middleware.GetUser` |
| `main.go` | Routes in the auth-required group: `GET/POST /auth/tokens`, `DELETE /auth/tokens/{id}` |

Tokens are never returned again after creation — only `prefix` and
`last_used_at` are shown — consistent with how API keys are masked elsewhere.

### Phase 2 — Token management UI (frontend)

Mirror the passkey UI on `frontend/src/pages/AccountPage.tsx`:

- `src/types/tokens.ts`, `src/api/tokens.ts`, `src/hooks/useTokens.ts`
  (React Query list/create/delete with `successToast`/`errorToast`).
- New "API Tokens" card on `AccountPage.tsx`: table (name, prefix, last used,
  created, revoke), a "Create token" modal, and a **create-once reveal** dialog
  with copy-to-clipboard (the secret is shown exactly once).
- Styling per `docs/style-guide.md` (button / label / table conventions).

### Phase 3 — Embed MCP over HTTP (backend)

- Build the `mcp-server` module: tools against the `Doer` interface, the
  standalone stdio binary (HTTP-client `Doer`), and an HTTP client supporting
  `CLAWORC_TOKEN` bearer auth (and username/password fallback).
- New `control-plane/internal/mcpserver/`:
  - `doer.go` — in-process `Doer`: builds an `*http.Request`, replays the
    captured auth header, calls `router.ServeHTTP`, returns status + body.
  - `server.go` — `NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server { … })`
    capturing auth per session and `tools.Register(s, doer)`.
- `main.go`: build the chi router into a variable, then mount
  `r.Handle("/mcp", mcpserver.Handler(r))` behind `RequireAuth`. Add the
  `mcp-server` require + `replace` directive.

### Phase 4 — Docs / CI / wiring

- This document + `mcp-server/README.md`: setup via
  `claude mcp add --transport http claworc https://…/mcp --header "Authorization: Bearer …"`,
  plus token-creation steps.
- `CLAUDE.md` architecture note + the new `mcp-server/` component. Helm needs
  nothing new (the `/mcp` route rides the control plane's existing ingress) — a
  one-line confirmation only.
- CI: a `.github/workflows/mcp-server.yml` (gofmt + vet + build + test); the
  control-plane workflow already covers its own tests.

## Testing

- **Unit:** token hashing/lookup; middleware bearer path (valid / expired /
  unknown); token handlers (create-once, list, revoke; cross-user isolation).
- **In-process Doer:** table test driving real tools through the router with an
  authenticated user — assert role enforcement (admin vs non-admin) and auth
  header replay.
- **MCP e2e:** in-memory MCP client over the `Doer`, against an httptest
  control-plane router.
- **Frontend:** typecheck + build.

## Risks

- **SDK auth capture:** confirm `getServer(*http.Request)` reliably exposes the
  inbound request for the streamable transport before wiring (verify against the
  SDK's `streamable.go`).
- **Cross-module `replace`:** ensure CI builds the control plane with the
  monorepo replace directive.
- **No double-auth surprises:** the replayed request re-runs `RequireAuth` with
  the same bearer token, so authorization matches a real external call.

## Sequencing

Phases 1 → 2 → 3 are the critical path; commit per phase. Phase 1 alone makes
bearer auth usable; Phase 3 delivers the web-connectable `/mcp` endpoint.

## Client setup (target end state)

1. In the Claworc dashboard → Account → API Tokens, create a token and copy it.
2. Register the connector:

   ```bash
   claude mcp add --transport http claworc https://claworc.example.com/mcp \
     --header "Authorization: Bearer claworc_pat_xxx"
   ```

   The same command works from Claude Code on the web and locally.
