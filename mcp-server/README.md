# claworc-mcp — Claworc MCP Server

A standalone [Model Context Protocol](https://modelcontextprotocol.io/) server
that wraps the Claworc control-plane REST API, letting an LLM client (Claude
Code, Claude Desktop, etc.) fully manage a Claworc deployment over stdio.

## Build

```bash
cd mcp-server
go build -o claworc-mcp .
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `CLAWORC_URL` | `http://localhost:8000` | Base URL of the Claworc control plane |
| `CLAWORC_TOKEN` | — | Bearer API token (`claworc_pat_…`); preferred auth method. Create one in the dashboard under Account → API Tokens. |
| `CLAWORC_USERNAME` | — | Login username (fallback when `CLAWORC_TOKEN` is not set) |
| `CLAWORC_PASSWORD` | — | Login password (fallback when `CLAWORC_TOKEN` is not set) |
| `CLAWORC_INSECURE` | `false` | Set to `true` to skip TLS certificate verification |
| `CLAWORC_TIMEOUT` | `60s` | Per-request timeout (Go duration, e.g. `30s`, `2m`) |

## Usage (stdio transport)

Register with Claude Code using a bearer token (recommended):

```bash
CLAWORC_TOKEN=claworc_pat_xxx \
CLAWORC_URL=https://claworc.example.com \
claude mcp add --transport stdio claworc ./claworc-mcp
```

Or using username/password (legacy):

```bash
CLAWORC_URL=https://claworc.example.com \
CLAWORC_USERNAME=admin \
CLAWORC_PASSWORD=secret \
claude mcp add --transport stdio claworc ./claworc-mcp
```

For remote (Streamable HTTP) access from Claude Code on the web, the control
plane exposes an embedded `/mcp` endpoint instead. See `docs/mcp-server.md`.

## Tools

The server registers over 60 typed tools covering:

- **Instances** — list, create, start, stop, restart, clone, update image, get/update config, stats, SSH status, logs, provider access
- **Providers** — list, create, update, delete, sync models, usage stats, catalog
- **Settings** — get/update global settings
- **Users** — list, create, delete, update role, reset password, manage team/instance assignments
- **Teams** — list, create, update, delete, manage members and provider whitelist
- **Skills** — list, deploy
- **Backups** — create, list, get, delete, restore, list schedules
- **Kanban** — list/get/create boards and tasks, start/stop tasks
- **Background tasks** — list, get, cancel
- **System** — orchestrator status, audit logs, shared folders
- **`claworc_request`** — escape-hatch tool for any endpoint not covered above
