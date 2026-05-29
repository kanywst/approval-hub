package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func recordingSender(out *[]Action) Sender {
	return func(a Action) tea.Cmd {
		*out = append(*out, a)
		return nil
	}
}

func TestUpdate_InitialPendingPopulates(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	got, _ := m.Update(InitialPendingMsg{
		{ID: "p1", SessionID: "s1", ToolName: "Bash"},
		{ID: "p2", SessionID: "s2", ToolName: "Edit"},
	})
	gm := got.(Model)
	if len(gm.pending) != 2 {
		t.Errorf("got %d pending, want 2", len(gm.pending))
	}
}

func TestUpdate_AddPendingAppends(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	got, _ := m.Update(AddPendingMsg{
		ID: "p1", SessionID: "s1", ToolName: "Bash",
		ReceivedAt: time.Now(),
	})
	if len(got.(Model).pending) != 1 {
		t.Errorf("got %d pending, want 1", len(got.(Model).pending))
	}
}

func TestUpdate_RemovePending_ByID(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got, _ := m.Update(RemovePendingMsg{ID: "b"})
	gm := got.(Model)
	if len(gm.pending) != 2 || gm.pending[1].ID != "c" {
		t.Errorf("unexpected pending list: %+v", gm.pending)
	}
}

func TestUpdate_RemovePending_ClampsSelection(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "a"}, {ID: "b"}}
	m.selected = 1
	got, _ := m.Update(RemovePendingMsg{ID: "b"})
	if got.(Model).selected != 0 {
		t.Errorf("selected: got %d, want 0", got.(Model).selected)
	}
}

func TestUpdate_Y_SendsOnceAllow(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "p1", Matcher: "Bash(x)"}}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if len(acts) != 1 {
		t.Fatalf("got %d actions, want 1", len(acts))
	}
	a := acts[0]
	if a.Decision != "allow" || a.Scope != "once" {
		t.Errorf("got %+v, want once-allow", a)
	}
}

func TestUpdate_A_SendsPersistAllowWithMatcher(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "p1", Matcher: "Bash(npm:*)"}}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if len(acts) != 1 {
		t.Fatalf("got %d actions", len(acts))
	}
	a := acts[0]
	if a.Decision != "allow" || a.Scope != "persist" || a.Matcher != "Bash(npm:*)" {
		t.Errorf("got %+v", a)
	}
}

func TestUpdate_D_SendsPersistDeny(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "p1", Matcher: "Bash(rm:*)"}}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if acts[0].Decision != "deny" || acts[0].Scope != "persist" {
		t.Errorf("got %+v", acts[0])
	}
}

func TestUpdate_JK_Move(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	m.pending = []Pending{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if got.(Model).selected != 1 {
		t.Errorf("j: got %d, want 1", got.(Model).selected)
	}
	got, _ = got.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if got.(Model).selected != 0 {
		t.Errorf("k: got %d, want 0", got.(Model).selected)
	}
}

func TestUpdate_QHelpToggle(t *testing.T) {
	var acts []Action
	m := New(recordingSender(&acts))
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !got.(Model).helpMode {
		t.Error("? did not enable helpMode")
	}
	got, _ = got.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if got.(Model).helpMode {
		t.Error("? did not toggle helpMode off")
	}
}

func TestColorFor_StableAcrossCalls(t *testing.T) {
	a := colorFor("abc123def456")
	b := colorFor("abc123def456")
	if a.GetForeground() != b.GetForeground() {
		t.Error("colorFor is not stable for the same session id")
	}
}
