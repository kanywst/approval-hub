// Package tui implements approval-hub's terminal UI for resolving pending
// permission requests against a running broker daemon.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Pending mirrors the broker's pendingView projection.
type Pending struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	Cwd        string         `json:"cwd"`
	ToolName   string         `json:"tool_name"`
	ToolInput  map[string]any `json:"tool_input"`
	Matcher    string         `json:"matcher"`
	ReceivedAt time.Time      `json:"received_at"`
}

// Action is a TUI-originated request to the broker.
type Action struct {
	Kind     string
	ID       string
	Decision string
	Scope    string
	TTL      int
	Matcher  string
}

// Sender is invoked from Update to dispatch actions back to the broker.
type Sender func(Action) tea.Cmd

// Messages dispatched into the Bubble Tea program from outside the model.
type (
	InitialPendingMsg []Pending
	AddPendingMsg     Pending
	RemovePendingMsg  struct{ ID string }
	ErrMsg            struct{ Err error }
)

// Model is the Bubble Tea Model.
type Model struct {
	pending   []Pending
	selected  int
	width     int
	height    int
	send      Sender
	helpMode  bool
	ttlMode   bool
	ttlInput  string
	statusMsg string
	lastErr   error
}

// New creates a Model wired to send.
func New(send Sender) Model {
	return Model{send: send}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case InitialPendingMsg:
		m.pending = []Pending(msg)
		m.clampSelection()
	case AddPendingMsg:
		m.pending = append(m.pending, Pending(msg))
	case RemovePendingMsg:
		m.removePending(msg.ID)
	case ErrMsg:
		m.lastErr = msg.Err
		m.statusMsg = "error: " + msg.Err.Error()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) removePending(id string) {
	for i, p := range m.pending {
		if p.ID == id {
			m.pending = append(m.pending[:i], m.pending[i+1:]...)
			break
		}
	}
	m.clampSelection()
}

func (m *Model) clampSelection() {
	if m.selected >= len(m.pending) {
		m.selected = len(m.pending) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpMode {
		if k := msg.String(); k == "?" || k == "q" || k == "esc" {
			m.helpMode = false
		}
		return m, nil
	}
	if m.ttlMode {
		return m.handleTTLKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.helpMode = true
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.pending)-1 {
			m.selected++
		}
	case "y":
		if cur, ok := m.current(); ok {
			return m, m.send(Action{
				Kind: "resolve", ID: cur.ID,
				Decision: "allow", Scope: "once",
			})
		}
	case "n":
		if cur, ok := m.current(); ok {
			return m, m.send(Action{
				Kind: "resolve", ID: cur.ID,
				Decision: "deny", Scope: "once",
			})
		}
	case "a":
		if cur, ok := m.current(); ok {
			return m, m.send(Action{
				Kind: "resolve", ID: cur.ID,
				Decision: "allow", Scope: "persist", Matcher: cur.Matcher,
			})
		}
	case "d":
		if cur, ok := m.current(); ok {
			return m, m.send(Action{
				Kind: "resolve", ID: cur.ID,
				Decision: "deny", Scope: "persist", Matcher: cur.Matcher,
			})
		}
	case "t":
		if _, ok := m.current(); ok {
			m.ttlMode = true
		}
	}
	return m, nil
}

func (m Model) handleTTLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		ttl, err := strconv.Atoi(m.ttlInput)
		m.ttlMode = false
		m.ttlInput = ""
		if err == nil && ttl > 0 {
			if cur, ok := m.current(); ok {
				return m, m.send(Action{
					Kind:     "resolve",
					ID:       cur.ID,
					Decision: "allow",
					Scope:    "ttl",
					TTL:      ttl,
					Matcher:  cur.Matcher,
				})
			}
		}
		return m, nil
	case "esc":
		m.ttlMode = false
		m.ttlInput = ""
	case "backspace":
		if len(m.ttlInput) > 0 {
			m.ttlInput = m.ttlInput[:len(m.ttlInput)-1]
		}
	default:
		if r := msg.String(); len(r) == 1 && r[0] >= '0' && r[0] <= '9' {
			m.ttlInput += r
		}
	}
	return m, nil
}

func (m Model) current() (Pending, bool) {
	if m.selected < 0 || m.selected >= len(m.pending) {
		return Pending{}, false
	}
	return m.pending[m.selected], true
}

// Pending returns the current pending list (exported for tests).
func (m Model) Pending() []Pending { return m.pending }

// View implements tea.Model.
func (m Model) View() string {
	if m.helpMode {
		return helpView()
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(
		fmt.Sprintf("approval-hub  pending: %d", len(m.pending))))
	b.WriteString("\n\n")
	for i, p := range m.pending {
		cursor := "  "
		if i == m.selected {
			cursor = "> "
		}
		row := fmt.Sprintf("%s[%s] %s  %s  %s",
			cursor,
			shortID(p.SessionID),
			toolBrief(p),
			truncate(p.Cwd, 30),
			p.ReceivedAt.Format("15:04:05"),
		)
		b.WriteString(colorFor(p.SessionID).Render(row) + "\n")
	}
	b.WriteString("\n")
	if cur, ok := m.current(); ok {
		b.WriteString(detailView(cur))
	}
	b.WriteString("\n")
	if m.ttlMode {
		fmt.Fprintf(&b, "TTL seconds (allow): %s_\n", m.ttlInput)
	}
	b.WriteString(barStyle.Render(
		"y once  n deny  a persist  d deny-persist  t ttl  ? help  q quit"))
	if m.statusMsg != "" {
		b.WriteString("\n" + m.statusMsg)
	}
	return b.String()
}

func detailView(p Pending) string {
	var b strings.Builder
	fmt.Fprintf(&b, "session_id: %s\n", p.SessionID)
	fmt.Fprintf(&b, "tool:       %s\n", p.ToolName)
	if cmd, ok := p.ToolInput["command"].(string); ok {
		fmt.Fprintf(&b, "command:    %s\n", cmd)
	}
	if fp, ok := p.ToolInput["file_path"].(string); ok {
		fmt.Fprintf(&b, "file:       %s\n", fp)
	}
	fmt.Fprintf(&b, "cwd:        %s\n", p.Cwd)
	fmt.Fprintf(&b, "matcher:    %s\n", p.Matcher)
	return b.String()
}

func toolBrief(p Pending) string {
	switch p.ToolName {
	case "Bash":
		if cmd, ok := p.ToolInput["command"].(string); ok {
			return "Bash: " + truncate(cmd, 30)
		}
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		if fp, ok := p.ToolInput["file_path"].(string); ok {
			return p.ToolName + ": " + truncate(fp, 30)
		}
	}
	return p.ToolName
}

func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return s[:n-1] + "..."
}

func helpView() string {
	return `approval-hub TUI

  y     allow this request only (no learning)
  n     deny this request only
  a     allow + learn matcher (persist)
  d     deny + learn matcher (persist)
  t     allow with TTL (you'll be prompted for seconds)
  k/up  move selection up
  j/dn  move selection down
  ?     toggle this help
  q     quit (broker keeps running)
`
}
