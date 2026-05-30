# approval-hub

Aggregate Claude Code permission prompts from multiple sessions into one
terminal UI. Pair with the [approval-hub plugin] for Claude Code; the
plugin embeds the broker's bearer token into `hooks/hooks.json` at
install time.

## Install

```bash
brew install kanywst/tap/approval-hub
# or
go install github.com/kanywst/approval-hub/cmd/approval-hub@latest
```

## Quick start

```bash
# 1. start the broker daemon (foreground)
approval-hub serve

# 2. in another terminal, attach the TUI
approval-hub attach

# 3. in any Claude Code session, install the plugin
/plugin install kanywst-plugins/approval-hub
/approval-hub:install
```

The plugin's PreToolUse hook now routes every permission prompt to the
broker. The TUI shows them in one place; press `y` / `n` / `a`
(allow + learn) / `d` (deny + learn) to resolve. Learned matchers
follow Claude Code's permission rule syntax (`Bash(npm test:*)`) so they
can be exported to `settings.json` later.

## Commands

- `approval-hub serve` — run the broker daemon in the foreground
- `approval-hub attach` — open the TUI client
- `approval-hub list` — list learned matcher rules
- `approval-hub revoke <id>` — delete a learned rule
- `approval-hub rotate` — rotate the bearer token
- `approval-hub doctor` — diagnose the install

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the broker / TUI / plugin
data flow and design rationale.

## Security

See [SECURITY.md](SECURITY.md) for the threat model and mitigations.

## License

MIT. See [LICENSE](LICENSE).

[approval-hub plugin]: https://github.com/kanywst/claude-code-plugins/tree/main/plugins/approval-hub
