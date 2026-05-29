// Package engine implements the broker's tool-input to matcher translation.
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// MatcherFor produces a permission-rule-syntax matcher for a tool invocation.
// The output format matches Claude Code's `if` field syntax so learned rules
// can also be exported to settings.json.
func MatcherFor(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "Bash":
		cmd, _ := toolInput["command"].(string)
		return bashMatcher(cmd)
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		if path, ok := toolInput["file_path"].(string); ok && path != "" {
			return fmt.Sprintf("%s(%s)", toolName, path)
		}
	}
	return fmt.Sprintf("%s(%s)", toolName, inputHash(toolInput))
}

// bashMatcher returns "Bash(<root>:*)" where root is the first token after
// stripping leading "KEY=VALUE" env assignments. This mirrors how Claude Code
// evaluates the `if` field for Bash, which discards env prefixes before
// matching.
func bashMatcher(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	for cmd != "" {
		head, rest, hasRest := strings.Cut(cmd, " ")
		if !isEnvAssignment(head) {
			break
		}
		if !hasRest {
			return "Bash(*)"
		}
		cmd = strings.TrimSpace(rest)
	}
	if cmd == "" {
		return "Bash(*)"
	}
	root, _, _ := strings.Cut(cmd, " ")
	return fmt.Sprintf("Bash(%s:*)", root)
}

func isEnvAssignment(token string) bool {
	if token == "" {
		return false
	}
	idx := strings.Index(token, "=")
	return idx > 0
}

func inputHash(in map[string]any) string {
	data, err := json.Marshal(in)
	if err != nil {
		return "*"
	}
	sum := sha256.Sum256(data)
	return "h_" + hex.EncodeToString(sum[:8])
}
