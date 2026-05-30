# approval-hub

> One TUI for every Claude Code permission prompt.

![CI](https://github.com/kanywst/approval-hub/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/github/license/kanywst/approval-hub)
![Release](https://img.shields.io/github/v/release/kanywst/approval-hub)

---

When you run multiple Claude Code sessions in parallel, every
PreToolUse permission prompt fires in a different terminal.
`approval-hub` aggregates them into a single TUI so you can resolve
every request from one place — and the learned rules apply to future
requests across all sessions.

```text
+- approval-hub ---------------------------------------------+
| pending (2):                                               |
|   > [a1b2c3d4] Bash: npm install               10:01:23    |
|     [e5f6g7h8] Edit: .env.local                10:01:25    |
|                                                            |
+------------------------------------------------------------+
| session_id: a1b2c3d4...                                    |
| tool:       Bash                                           |
| command:    npm install                                    |
| cwd:        ~/proj/foo                                     |
| matcher:    Bash(npm:*)                                    |
|                                                            |
| y once  n deny  a persist  d deny-persist  t ttl  ? help q |
+------------------------------------------------------------+
```

## Highlights

- **One TUI for every session.** Each session's PreToolUse hook posts
  to a local broker; the broker pushes pending requests over SSE to a
  single Bubble Tea TUI.
- **Matcher learning.** `a` / `d` records a permission rule
  (`Bash(npm test:*)`, `Edit(.env)`) so subsequent matching requests
  resolve in about 30 ms without prompting.
- **TTL rules.** `t` grants a one-hour pass instead of a forever rule.
- **Fails safe.** Kill the broker and Claude Code's hook receives a
  connection error; Claude Code falls back to its built-in prompt.
  Nothing breaks.
- **Local-only.** `127.0.0.1` listener, bearer token stored in `0600`
  files, no network exposure.

## Install

```bash
brew install kanywst/tap/approval-hub
```

Or, from source:

```bash
go install github.com/kanywst/approval-hub/cmd/approval-hub@latest
```

## Quick start

```bash
# 1. run the broker daemon (foreground)
approval-hub serve

# 2. in another terminal, attach the TUI
approval-hub attach

# 3. in any Claude Code session, install the plugin and bootstrap
/plugin install kanywst-plugins/approval-hub
/approval-hub:install
```

The plugin's PreToolUse hook now routes every permission prompt to the
broker. Spin up a second Claude Code session anywhere on the same
machine and its requests join the same TUI.

## TUI keybindings

| Key         | Action                                          |
| ----------- | ----------------------------------------------- |
| `y`         | allow once                                      |
| `n`         | deny once                                       |
| `a`         | allow and learn the matcher (persist)           |
| `d`         | deny and learn the matcher (persist)            |
| `t`         | allow with a TTL (prompted for seconds)         |
| `k` / `up`  | move selection up                               |
| `j` / `dn`  | move selection down                             |
| `?`         | toggle help                                     |
| `q`         | quit (broker keeps running)                     |

## CLI

| Command                    | Purpose                                |
| -------------------------- | -------------------------------------- |
| `approval-hub serve`       | run the broker daemon in the foreground|
| `approval-hub attach`      | open the TUI client                    |
| `approval-hub list`        | list learned matcher rules             |
| `approval-hub revoke <id>` | delete a learned rule                  |
| `approval-hub rotate`      | rotate the bearer token                |
| `approval-hub doctor`      | diagnose the install                   |

## How it works

```text
+------------------+      +------------------+
| claude session A |      | claude session B |
|  PreToolUse:http |      |  PreToolUse:http |
+---------+--------+      +---------+--------+
          | POST /decide            | POST /decide
          +-------------+-----------+
                        v
            +-----------------------+
            |  broker daemon        |
            |  127.0.0.1:17456      |
            |  matcher store (JSONL)|
            +-----------+-----------+
                        | SSE /events
                        v
            +-----------------------+
            |  TUI client           |
            |  approval-hub attach  |
            +-----------------------+
```

Cache hits return in single-digit milliseconds; the user never sees a
prompt for a rule they have already learned. See
[ARCHITECTURE.md](ARCHITECTURE.md) for component responsibilities and
the broker HTTP API.

## Security

`approval-hub` listens on `127.0.0.1` only and authenticates every
request with a `crypto/rand` 32-byte bearer token stored in `0600`
files. See [SECURITY.md](SECURITY.md) for the threat model.

## License

MIT — see [LICENSE](LICENSE).
