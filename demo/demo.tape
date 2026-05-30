Output assets/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1180
Set Height 640
Set Padding 20
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

Hide
Type `export TOKEN=$(jq -r .token "$HOME/Library/Application Support/approval-hub/config.json")`
Enter
Type "clear"
Enter
Show

Type "# approval-hub: one TUI for every Claude Code permission prompt"
Sleep 700ms
Enter
Sleep 600ms

Type "# queue two PreToolUse requests from two parallel Claude sessions"
Enter
Sleep 400ms

Type `curl -sS -X POST http://127.0.0.1:17456/decide -H "Authorization: Bearer $TOKEN" -d '{"session_id":"sess-web","cwd":"~/dev.to","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"npm install"}}' >/dev/null &`
Enter
Sleep 700ms

Type `curl -sS -X POST http://127.0.0.1:17456/decide -H "Authorization: Bearer $TOKEN" -d '{"session_id":"sess-api","cwd":"~/api","hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":".env.local"}}' >/dev/null &`
Enter
Sleep 900ms

Type "clear"
Enter
Sleep 300ms

Type "approval-hub attach"
Enter
Sleep 2200ms

Type "j"
Sleep 600ms
Type "k"
Sleep 600ms

Type "a"
Sleep 1500ms

Type "d"
Sleep 1500ms

Type "q"
Sleep 600ms

Type "clear"
Enter
Sleep 200ms

Type "# learned rules persist across sessions"
Enter
Sleep 400ms
Type "approval-hub list"
Enter
Sleep 2500ms
