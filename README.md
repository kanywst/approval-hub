# approval-hub

<img src="assets/logo.svg" alt="approval-hub" width="160" align="right">

> One TUI for every Claude Code permission prompt.

![CI](https://github.com/kanywst/approval-hub/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/github/license/kanywst/approval-hub)
![Release](https://img.shields.io/github/v/release/kanywst/approval-hub)

---

When you run multiple Claude Code sessions in parallel, every
PreToolUse permission prompt fires in a different terminal.
`approval-hub` aggregates them into a single TUI so you can resolve
every request from one place. Learned rules apply to future requests
across every session on the same machine.

<img src="assets/demo.gif" alt="approval-hub demo" width="900">

Two parallel Claude Code sessions queue PreToolUse requests; one TUI
resolves them. `a` persists a learned `Bash(npm:*)` allow rule, `d`
persists an `Edit(.env.local)` deny rule, and the next matching request
from any session resolves from the store without prompting. `t` grants
a TTL rule instead of a forever one.

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
files. If the broker is not running, Claude Code's hook gets a
connection error and falls back to its built-in prompt, so no session
is blocked by a daemon outage. See [SECURITY.md](SECURITY.md) for the
threat model.

## License

MIT. See [LICENSE](LICENSE).
