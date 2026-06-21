# MCP Server — Multi-Agent Implementation Plan

Execution plan for building the design in [`docs/mcp-server.md`](./mcp-server.md)
using a lead orchestrator plus parallel sub-agents on **Claude Sonnet 4.6** and
**Claude Haiku 4.5**. Branch: `claude/mcp-server-plan` (clean off `main`).

The orchestrator (this lead session) owns: freezing the contracts below,
dispatching sub-agents, integrating their output, running the full verification
gate, and committing per wave. Sub-agents do the implementation in isolated
worktrees and report back diffs + verification results.

---

## 1. Model assignment policy

Pick the cheapest model that can do the task correctly.

- **Sonnet 4.6** — security-sensitive code, cross-file integration, designing an
  interface/contract, anything where a wrong guess is expensive or where the
  task touches auth, the router, or module wiring.
- **Haiku 4.5** — mechanical, fully-specified work with a clear template and no
  design decisions: boilerplate CRUD, type/DTO files, list/table UI from an
  existing analog, docs, CI YAML, porting code that already exists.

Every task below is tagged `[Sonnet]` or `[Haiku]`. Dispatch with the `Agent`
tool's `model` parameter (`"sonnet"` / `"haiku"`), `isolation: "worktree"`, and
`run_in_background: true` for parallel waves.

---

## 2. Frozen contracts

These are fixed **before any sub-agent starts** so parallel work composes. No
sub-agent may change a signature here without the orchestrator updating this
section and notifying dependents.

### 2.1 Tool transport interface (`mcp-server/internal/tools`)

```go
type Result struct {
    Status int
    Body   []byte
}

// Doer executes an API call and returns the raw response. Auth is the Doer's
// responsibility (bearer header for HTTP; replayed request context in-process).
type Doer interface {
    // API targets the control plane under /api/v1.
    API(ctx context.Context, method, path string, query url.Values, body any) (*Result, error)
    // Raw targets an arbitrary path (escape-hatch tool).
    Raw(ctx context.Context, method, path string, query url.Values, body any) (*Result, error)
}

// Register adds every Claworc tool to s, bound to d.
func Register(s *mcp.Server, d Doer)
```

`body any` accepts a struct, a map, or `json.RawMessage` (sent verbatim).

### 2.2 Token REST API

| Method | Path | Body | Success |
|---|---|---|---|
| `POST` | `/api/v1/auth/tokens` | `{"name": string, "expires_in_days"?: number}` | `201` `{id,name,token,prefix,expires_at?,created_at}` — `token` returned **only here** |
| `GET` | `/api/v1/auth/tokens` | — | `200` `[{id,name,prefix,last_used_at?,expires_at?,created_at}]` |
| `DELETE` | `/api/v1/auth/tokens/{id}` | — | `204` |

All three live in the **auth-required** route group (self-service, not
admin-only), scoped to the caller via `middleware.GetUser`.

### 2.3 Token format & storage

- Plaintext: `claworc_pat_` + 32 random bytes hex (64 hex chars).
- Stored: `TokenHash = hex(sha256(plaintext))` only. Never store plaintext.
- `Prefix`: the literal `claworc_pat_` + first 6 hex chars (for display/identification).
- Lookup: strip `Bearer `, require `claworc_pat_` prefix, `sha256`, `GetAPITokenByHash`, reject if expired.

### 2.4 Embedded handler & env

- Embedded mount: `r.Handle("/mcp", mcpserver.NewHandler(r))` behind `RequireAuth`,
  where `func NewHandler(router http.Handler) http.Handler`.
- Standalone env vars: `CLAWORC_URL`, `CLAWORC_TOKEN` (preferred bearer),
  `CLAWORC_USERNAME`/`CLAWORC_PASSWORD` (fallback), `CLAWORC_INSECURE`, `CLAWORC_TIMEOUT`.

### 2.5 Frontend types (`frontend/src/types/tokens.ts`)

```ts
export interface APIToken {
  id: number; name: string; prefix: string;
  last_used_at?: string; expires_at?: string; created_at: string;
}
export interface APITokenCreated extends APIToken { token: string }
export interface CreateAPITokenRequest { name: string; expires_in_days?: number }
```

### 2.6 Reference material

The throwaway stdio prototype on branch `claude/mcp-server-build-j55lpc`
(`mcp-server/internal/tools/*.go`, `internal/client/client.go`) already contains
the ~65 tool definitions and an HTTP client. Sub-agents on the `mcp-server`
workstream should **read that branch and port**, not rewrite from scratch:
`git show claude/mcp-server-build-j55lpc:mcp-server/internal/tools/instances.go` etc.

---

## 3. Task DAG & waves

```
Wave 0  S1 [Sonnet] SDK spike ──┐
        (orchestrator freezes contracts §2)
                                │
Wave 1  T1 [Sonnet] backend token auth        ─┐
        T2 [Sonnet→Haiku] mcp-server module    │  (parallel, disjoint files)
        T3 [Haiku→Sonnet] frontend token UI    │
                                ────────────────┘
                                │ (T1 + T2 merged)
Wave 2  T4 [Sonnet] control-plane embedding /mcp
                                │
Wave 3  T5 [Haiku] docs   T6 [Haiku] CI   (parallel)
        T7 [Sonnet] integration + full verification + commit
```

---

## 4. Task specifications

### S1 — SDK spike: per-session request access `[Sonnet]` (Wave 0)
- **Goal:** confirm `mcp.NewStreamableHTTPHandler`'s `getServer func(*http.Request) *mcp.Server`
  reliably receives the inbound HTTP request (so we can capture the auth header
  per session). Read `streamable.go` in the SDK module cache; write a 30-line
  throwaway main that mounts a handler, logs the request header, and hits it.
- **Output:** a short note in this file's §6 confirming the hook, or an
  alternative (e.g. context value) if not. **Blocks T4.**
- **Verify:** the throwaway prints the forwarded header.
- **RESULT (resolved):** `h.getServer(req)` is called inside
  `StreamableHTTPHandler.ServeHTTP` (SDK v1.6.1 `streamable.go:401`) with the
  live `*http.Request` on session creation. T4 captures the `Authorization`
  header in the `getServer` factory and binds it into the in-process `Doer`.
  Since the outer `/mcp` route and the replayed request both re-run
  `RequireAuth`, token revocation/expiry is honored per request. No alternative
  needed.

### T1 — Backend API-token auth `[Sonnet]` (Wave 1)
- **Owns (exclusive):** `internal/database/models/models.go`,
  `internal/database/models.go`, `internal/database/database.go`,
  `internal/database/migrations/migration_00001_baseline.go`,
  `internal/auth/auth.go`, `internal/middleware/auth.go`,
  `internal/handlers/tokens.go` (new), `internal/handlers/tokens_test.go` (new),
  `internal/middleware/auth_test.go`; **and `main.go` token routes only**.
- **Steps:**
  1. `APIToken` model (§2.3 fields) + alias + register in `AutoMigrateAll`.
  2. DB helpers: `CreateAPIToken`, `ListAPITokensByUser`, `GetAPITokenByHash`,
     `DeleteAPIToken(id, userID)`, `TouchAPITokenLastUsed`.
  3. `auth.GenerateAPIToken() (plain, hash, prefix string)`.
  4. `RequireAuth`: bearer path per §2.3 before the cookie path; preserves the
     existing context shape; throttle `TouchAPITokenLastUsed` (e.g. skip if
     `last_used_at` < 1 min old).
  5. `handlers/tokens.go`: List/Create/Delete per §2.2, mirroring the WebAuthn
     handlers; create returns the plaintext once.
  6. `main.go`: add the three routes in the auth-required group.
  7. Tests: middleware bearer (valid/expired/unknown/non-pat), handler
     create-once + cross-user isolation.
- **Verify:** `cd control-plane && gofmt -l . && go vet ./... && go build ./... && go test ./internal/...`

### T2 — `mcp-server` module `[Sonnet then Haiku]` (Wave 1)
Single workstream, two sub-tasks (same worktree, sequential):
- **T2a `[Sonnet]`** — module scaffold: `go.mod` (`github.com/gluk-w/claworc/mcp-server`),
  `internal/tools/tools.go` (the `Doer` interface §2.1, `Register`, result
  formatting helpers), `internal/tools/raw.go`, `internal/client/client.go`
  (HTTP `Doer`: bearer `CLAWORC_TOKEN` or password login fallback, 401 re-auth),
  `main.go` (stdio transport, env config), and the in-memory MCP client test
  harness. Define the exact helper signatures T2b depends on.
- **T2b `[Haiku]`** — port the per-domain tool files from the prototype branch
  (instances, providers, settings, users, teams, skills, backups, kanban, tasks,
  system) to the frozen helper signatures. Pure mechanical translation.
- **Owns (exclusive):** all of `mcp-server/**` (new module — no overlap).
- **Verify:** `cd mcp-server && gofmt -l . && go vet ./... && go build ./... && go test ./...`
  (≥60 tools registered; list/call/validation/raw tests pass).

### T3 — Frontend token UI `[Haiku then Sonnet]` (Wave 1)
- **T3a `[Haiku]`** — `src/types/tokens.ts` (§2.5), `src/api/tokens.ts`
  (`listAPITokens`/`createAPIToken`/`deleteAPIToken`), `src/hooks/useTokens.ts`
  (React Query list/create/delete with `successToast`/`errorToast`). Follow the
  settings/webauthn analogs exactly.
- **T3b `[Sonnet]`** — add the "API Tokens" card to `src/pages/AccountPage.tsx`:
  table (name, prefix, last used, created, revoke), create modal, and a
  **create-once reveal** dialog with copy-to-clipboard. Reuse existing
  modal/button/toast components; style per `docs/style-guide.md`.
- **Owns (exclusive):** the four files above. (Only `AccountPage.tsx` is
  pre-existing; no other task touches it.)
- **Verify:** `cd control-plane/frontend && npm ci && npm run build` (tsc + vite).

### T4 — Control-plane embedding `[Sonnet]` (Wave 2, needs T1+T2+S1)
- **Owns (exclusive):** `internal/mcpserver/doer.go`,
  `internal/mcpserver/server.go`, `internal/mcpserver/*_test.go` (new),
  `control-plane/go.mod`+`go.sum`; **and `main.go` `/mcp` mount + router var**.
- **Steps:**
  1. `go.mod`: `require github.com/gluk-w/claworc/mcp-server v0.0.0` +
     `replace github.com/gluk-w/claworc/mcp-server => ../mcp-server`.
  2. `doer.go`: in-process `Doer` — build `*http.Request`, replay captured auth
     header, `router.ServeHTTP(rec, req)`, return `*tools.Result`.
  3. `server.go`: `NewHandler(router http.Handler) http.Handler` wrapping
     `mcp.NewStreamableHTTPHandler` whose `getServer` captures the request auth,
     builds a `*mcp.Server`, and calls `tools.Register(s, doer)`.
  4. `main.go`: assign the chi router to a variable; mount `/mcp` behind
     `RequireAuth`.
  5. Test: drive a tool through the in-process Doer with an authenticated user;
     assert admin vs non-admin enforcement and header replay.
- **Verify:** `cd control-plane && go build ./... && go test ./internal/mcpserver/...`

### T5 — Docs `[Haiku]` (Wave 3)
- Update `mcp-server/README.md` (HTTP + token setup), `CLAUDE.md`
  (add `mcp-server/` component + `/mcp` architecture note), one-line Helm
  confirmation, and flip `docs/mcp-server.md` status to "implemented".
- **Verify:** prose only; no build.

### T6 — CI `[Haiku]` (Wave 3)
- `.github/workflows/mcp-server.yml`: gofmt-check + `go vet` + `go build` +
  `go test` for the `mcp-server` module, triggered on `mcp-server/**`.
- **Verify:** `yamllint`/manual; orchestrator confirms it matches the
  control-plane workflow conventions.

### T7 — Integration & verification `[Sonnet]` (Wave 3, needs all)
- Merge worktrees; resolve any `main.go`/`go.mod` overlap.
- Run the **full gate** (§5). Fix cross-task breakage (likely: control-plane
  building the embedded module via the `replace`).
- End-to-end smoke: build control-plane, start it with `CLAWORC_AUTH_DISABLED`
  or a seeded admin, create a token via the API, call `/mcp` with an MCP client
  (or the SDK test client) using `Authorization: Bearer`, list instances.
- Commit per wave already done; T7 makes the final integration commit and pushes.

---

## 5. Verification gate (orchestrator runs before each merge & at the end)

```bash
# Backend (control plane)
cd control-plane && gofmt -l . && go vet ./... && go build ./... && go test ./internal/...
# MCP module
cd ../mcp-server && gofmt -l . && go vet ./... && go build ./... && go test ./...
# Embedded build (after T4's replace directive)
cd ../control-plane && go build ./...
# Frontend
cd frontend && npm ci && npm run build
```

A task is **done** only when its own verify command passes *and* it hasn't
broken the gate.

---

## 6. File-ownership matrix (conflict avoidance)

| Path | Owner | Wave |
|---|---|---|
| `control-plane/internal/{database,auth,middleware}/**`, `handlers/tokens*.go` | T1 | 1 |
| `control-plane/main.go` — token routes | T1 | 1 |
| `control-plane/main.go` — router var + `/mcp` mount | T4 | 2 |
| `control-plane/internal/mcpserver/**`, `control-plane/go.mod`,`go.sum` | T4 | 2 |
| `mcp-server/**` | T2 | 1 |
| `control-plane/frontend/src/{types,api,hooks}/tokens*`, `pages/AccountPage.tsx` | T3 | 1 |
| `docs/**`, `CLAUDE.md`, `mcp-server/README.md`, `helm/**` note | T5 | 3 |
| `.github/workflows/mcp-server.yml` | T6 | 3 |

The only file two tasks touch is `main.go` (T1 then T4) — sequenced across waves,
so no concurrent edit. Everything within a wave is disjoint → safe to parallelize.

---

## 7. Orchestration runbook

1. **Wave 0:** run S1; freeze §2 (already drafted). Commit the spike note.
2. **Wave 1:** dispatch T1, T2, T3 in parallel (`run_in_background`, worktrees).
   Within T2 run T2a then hand its worktree to T2b via `SendMessage`; same for
   T3a→T3b. As each returns, run its verify, integrate to the branch, commit.
3. **Wave 2:** after T1+T2 are on the branch, run T4. Integrate, run gate.
4. **Wave 3:** dispatch T5, T6 in parallel; run T7 to close out, push.
5. If a sub-agent's diff fails the gate or deviates from §2, the orchestrator
   bounces it back via `SendMessage` with the specific failure — it does not
   silently patch around it.

---

## 8. Definition of done

- All §5 gate commands green; `replace`-built control plane embeds the module.
- A bearer token created in the UI authenticates a real MCP call to `/mcp`.
- Standalone stdio binary still builds and works with `CLAWORC_TOKEN`.
- Docs/CI updated; branch pushed. No PR unless explicitly requested.

## 9. Risks

- **S1 outcome** gates T4's auth-capture approach — resolve in Wave 0.
- **Cross-module `replace`** must be reflected in CI so the control-plane build
  resolves `../mcp-server`.
- **No privilege bypass:** the in-process Doer replays the caller's bearer
  token, so `/mcp` calls re-run `RequireAuth` and all role checks unchanged.
- **Worktree merge** of `main.go`/`go.mod` is the one manual integration point;
  kept trivial by the wave sequencing in §6.
