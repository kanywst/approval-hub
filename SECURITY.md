# Security

## Threat model

approval-hub runs as the same OS user as the Claude Code session it
brokers for, on the local machine only.

### In scope

- An attacker on the same network attempting to send `/decide` requests
  to the broker → mitigated by binding only to `127.0.0.1`.
- A passive observer on the local network → mitigated by `127.0.0.1`
  only; no TLS is needed.
- A program owned by a different OS user reading the token or sending
  requests → mitigated by `0600` permissions on `config.json` and
  `hooks/hooks.json`.

### Out of scope

- Other processes running as the same OS user. They can read
  `config.json` and impersonate the broker or the TUI. Defense at this
  layer is the OS user account itself.
- Root-level malware. If the system is rooted, all bets are off.
- Bugs in Claude Code itself.

## Mitigations

- Bind only to `127.0.0.1`; never to `0.0.0.0` or a Unix socket exposed
  to other users.
- Bearer token (`crypto/rand` 32 bytes, hex-encoded with `ahub_` prefix).
- `0600` on `config.json`, `hooks/hooks.json`, `learned-rules.jsonl`,
  and `approval-hub.pid`.
- Constant-time token comparison (`crypto/subtle.ConstantTimeCompare`).
- Token rotation via `approval-hub rotate` (also rewrites the plugin's
  `hooks.json`).
- Rate limiter (`rate_limit_per_second`) and max-pending bound
  (`max_pending_requests`) to slow brute-force or DoS attempts even
  from the same user.

## Reporting vulnerabilities

Please open a GitHub issue tagged `security` or email the maintainer
listed in the README. Do not disclose publicly until a fix is released.
