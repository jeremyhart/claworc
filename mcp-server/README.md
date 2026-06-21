# Claworc MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
the Claworc control-plane API as tools, so an LLM client (Claude Desktop, Claude
Code, or any MCP-compatible agent) can **fully manage a Claworc deployment** —
create and operate OpenClaw instances, manage users, teams, LLM providers,
skills, backups, Kanban boards, and global settings.

It speaks MCP over **stdio** and authenticates to the control plane with a
username/password, transparently maintaining the session cookie (and
re-authenticating when it expires).

## How it works

```
LLM client  <--stdio/MCP-->  claworc-mcp  <--HTTPS/session cookie-->  Claworc control plane
```

The server is a thin, stateless wrapper over the existing REST API under
`/api/v1`. Every tool maps to one or more endpoints; permissions are still
enforced by the control plane based on the authenticated user's role (admin /
team-manager / user). Logging in as an **admin** unlocks the full tool surface.

## Build

```bash
cd mcp-server
go build -o claworc-mcp .
```

This produces a single static binary with no runtime dependencies.

## Configuration

The server is configured entirely through environment variables:

| Variable           | Default                 | Description                                              |
| ------------------ | ----------------------- | -------------------------------------------------------- |
| `CLAWORC_URL`      | `http://localhost:8000` | Base URL of the control plane.                           |
| `CLAWORC_USERNAME` | —                       | Login username. Omit only if the control plane has auth disabled. |
| `CLAWORC_PASSWORD` | —                       | Login password.                                          |
| `CLAWORC_INSECURE` | `false`                 | Set to `true` to skip TLS verification (self-signed certs). |
| `CLAWORC_TIMEOUT`  | `60s`                   | Per-request timeout (Go duration string).               |

## Use with Claude Code

```bash
claude mcp add claworc \
  --env CLAWORC_URL=https://claworc.example.com \
  --env CLAWORC_USERNAME=admin \
  --env CLAWORC_PASSWORD=your-password \
  -- /path/to/claworc-mcp
```

## Use with Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "claworc": {
      "command": "/path/to/claworc-mcp",
      "env": {
        "CLAWORC_URL": "https://claworc.example.com",
        "CLAWORC_USERNAME": "admin",
        "CLAWORC_PASSWORD": "your-password"
      }
    }
  }
}
```

## Run with Docker

```bash
docker build -t claworc/mcp-server ./mcp-server

docker run --rm -i \
  -e CLAWORC_URL=https://claworc.example.com \
  -e CLAWORC_USERNAME=admin \
  -e CLAWORC_PASSWORD=your-password \
  claworc/mcp-server
```

> The container reads/writes MCP messages on stdio, so run it with `-i`
> (interactive) and wire the MCP client's `command` to the `docker run …`
> invocation.

## Tools

All tools are prefixed `claworc_`. Highlights:

**Instances** — `list_instances`, `get_instance`, `create_instance`,
`update_instance`, `delete_instance`, `start_instance`, `stop_instance`,
`restart_instance`, `clone_instance`, `update_instance_image`,
`get_instance_config`, `update_instance_config`, `get_instance_stats`,
`get_instance_ssh_status`, `get_instance_logs`, `list_instance_providers`.

**LLM providers** — `list_providers`, `create_provider`, `update_provider`,
`delete_provider`, `sync_provider_models`, `get_usage_stats`,
`get_provider_catalog`.

**Settings** — `get_settings`, `update_settings`.

**Users** — `list_users`, `create_user`, `delete_user`, `update_user_role`,
`reset_user_password`, `get_user_teams`, `get_user_instances`,
`set_user_instances`.

**Teams** — `list_teams`, `create_team`, `update_team`, `delete_team`,
`list_team_members`, `set_team_member`, `remove_team_member`,
`get_team_providers`, `set_team_providers`.

**Skills** — `list_skills`, `deploy_skill`.

**Backups** — `create_backup`, `list_instance_backups`, `list_backups`,
`get_backup`, `delete_backup`, `restore_backup`, `list_backup_schedules`.

**Kanban** — `list_kanban_boards`, `get_kanban_board`, `create_kanban_board`,
`get_kanban_task`, `create_kanban_task`, `start_kanban_task`, `stop_kanban_task`.

**Tasks (background jobs)** — `list_tasks`, `get_task`, `cancel_task`.

**System** — `orchestrator_status`, `get_audit_logs`, `list_shared_folders`.

**Escape hatch** — `claworc_request` makes an arbitrary authenticated request
(`method`, `path`, `query`, `body`) to any control-plane endpoint, covering the
full API surface for operations without a dedicated typed tool.

## Tests

```bash
go test ./...
```

The test suite stands up a mock control plane and drives the server through a
real in-memory MCP client to verify login, tool registration, successful calls,
input validation, and the raw request escape hatch.
