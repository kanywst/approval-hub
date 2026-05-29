package tui

import (
	"crypto/sha1"

	"github.com/charmbracelet/lipgloss"
)

var sessionColors = []string{
	"#ff5555", "#ffb86c", "#f1fa8c", "#50fa7b",
	"#8be9fd", "#bd93f9", "#ff79c6", "#a4d9ff",
}

// colorFor maps a session ID to a stable lipgloss foreground style.
func colorFor(sessionID string) lipgloss.Style {
	if sessionID == "" {
		return lipgloss.NewStyle()
	}
	h := sha1.Sum([]byte(sessionID))
	idx := int(h[0]) % len(sessionColors)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(sessionColors[idx]))
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#bd93f9"))
	barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272a4"))
)
