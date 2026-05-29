# Architecture

approval-hub is a local HTTP broker daemon plus a Bubble Tea TUI client,
both packaged in a single Go binary (`approval-hub`). A thin Claude Code
plugin distributed separately wires each Claude Code session's `PreToolUse`
hook to the broker via `type: "http"`.

## Process layout

```text
+------------------+        +------------------+
| claude session A |        | claude session B |
|  PreToolUse:http |        |  PreToolUse:http |
+--------+---------+        +--------+---------+
         |                           |
         | POST /decide              | POST /decide
         | Bearer <token>            | Bearer <token>
         +-------------+-------------+
                       v
            +---------------------+
            |  broker daemon      |
            |  127.0.0.1:17456    |
            |                     |
            |  - decision engine  |
            |  - matcher store    |
            |  - pending queue    |
            |  - SSE /events      |
            +---------+-----------+
                      | SSE /events
                      v
            +---------------------+
            |  TUI client         |
            |  approval-hub attach|
            +---------------------+
```

Three processes are loosely coupled over HTTP and SSE. If the TUI or the
daemon dies, Claude sessions keep working: the hook receives a non-2xx /
connection error and Claude Code falls back to its built-in interactive
prompt. This is guaranteed by Claude Code's hook semantics.

## Broker daemon

Single goroutine-safe HTTP server on `127.0.0.1`.

- `POST /decide` — PreToolUse entry. Generates a permission-rule-syntax
  matcher (`Bash(npm test:*)`, `Edit(/path/to/file)`, ...) from the tool
  input, looks it up in the matcher store. Cache hit returns immediately;
  cache miss enqueues a pending request and waits up to
  `ui_timeout_seconds` for a TUI decision, after which it returns `defer`.
- `GET /events` — Server-Sent Events stream. Publishes
  `pending_added` / `pending_resolved` / `pending_expired` / `rule_added`
  / `rule_revoked`. Slow subscribers drop events when their buffer fills.
- `GET /pending`, `POST /pending/{id}/resolve` — list and resolve.
- `GET /rules`, `DELETE /rules/{id}` — inspect and tombstone matchers.
- `POST /rotate-token` — generate a new bearer token and persist it.

Bearer-token middleware in front of every route. Rate limiter uses a
per-second token bucket sized by `rate_limit_per_second`.

## TUI client

Bubble Tea Model with an SSE consumer goroutine. Reconnects with
exponential backoff up to 30 s. Decisions go back to the broker as
`POST /pending/{id}/resolve` with `scope` of `once` (no learning),
`persist` (matcher appended to the store), or `ttl` (matcher with an
`expires_at`).

Session IDs are SHA-1 hashed to one of 8 ANSI colors so requests from
the same session group visually.

## Matcher store

Append-only JSONL log at `${data_dir}/learned-rules.jsonl`. Each line is
either a `rule_added` record (with optional `expires_at`) or a
`rule_revoked` tombstone referencing a previous rule's ID. The broker
reconstructs an in-memory map on startup.

Matcher strings follow Claude Code's `if` field syntax so the rule set
can be exported to `settings.json`'s `permissions.allow` list when the
broker is uninstalled.

## Failure modes

- **broker not running** — hook receives connection refused → Claude
  Code falls back to manual prompt
- **broker crashes mid-pending** — hook hits its own `timeout` →
  Claude Code falls back
- **TUI client crashes** — broker keeps queueing pending; next attach
  replays via `GET /pending` and SSE
- **pending queue full** — broker returns 503; Claude Code falls back
- **disk full (store)** — `AddRule` errors; resolve still returns the
  decision in-memory

## Configuration

`config.json` in the OS-standard data directory
(`~/Library/Application Support/approval-hub` on macOS,
`~/.config/approval-hub` on Linux, or `$APPROVAL_HUB_DATA`).

```json
{
  "port": 17456,
  "token": "ahub_<64hex>",
  "ui_timeout_seconds": 60,
  "max_pending_requests": 100,
  "rate_limit_per_second": 50
}
```

File permissions are `0600`. The token is embedded into the Claude Code
plugin's `hooks/hooks.json` by the `install` skill.
