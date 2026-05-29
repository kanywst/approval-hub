# approval-hub

複数の Claude Code セッションの PreToolUse 許可プロンプトを 1 画面 (TUI)
に集約する broker daemon と、それを呼ぶ Claude Code plugin の組合せ。

実装は本リポ内 `0-draft/approval-hub/` 配下。完成時に以下へ分割する。

- 本体 (daemon + TUI + matcher 学習 store): 別リポ `kanywst/approval-hub`
- plugin wrapper: `../../plugins/approval-hub/`

## 何を解くか

複数の Claude Code セッション (別ターミナル / tmux pane) を並走させると、
各セッションが個別に yes/no プロンプトを出して把握しきれない。これを 1
画面に集約し、`y` / `n` / `a` (allow always + matcher 学習) / `d`
(deny always) で捌けるようにする。

## 技術メモ

- PreToolUse hook の `type: "http"` で、各セッションから
  `http://127.0.0.1:<port>/decide` に POST
- payload の `session_id` + `cwd` でセッション識別
- broker が落ちてたら `"defer"` を返して従来の手動 prompt に escalate
  (壊れない)
- TUI: Bubble Tea (Go)
- セキュリティ: 127.0.0.1 限定 listen、Bearer token は env var
  (`APPROVAL_HUB_TOKEN`) を `allowedEnvVars` 経由で hook に渡す

### 返却 JSON 例

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow|deny|ask|defer",
    "permissionDecisionReason": "..."
  }
}
```

## 進捗

着手前。詳細仕様調査 → 設計ドキュメント → 実装計画分解 → 実装の順で進める。
