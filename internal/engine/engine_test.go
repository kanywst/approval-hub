package engine

import (
	"strings"
	"testing"
)

func TestMatcherFor_Bash(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"npm test", "Bash(npm:*)"},
		{"npm test --watch", "Bash(npm:*)"},
		{"rm -rf /tmp/foo", "Bash(rm:*)"},
		{"  git push origin main  ", "Bash(git:*)"},
		{"", "Bash(*)"},
		{"FOO=bar BAZ=qux npm test", "Bash(npm:*)"},
		{"FOO=bar", "Bash(*)"},
	}
	for _, c := range cases {
		got := MatcherFor("Bash", map[string]any{"command": c.cmd})
		if got != c.want {
			t.Errorf("MatcherFor Bash %q: got %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestMatcherFor_Edit(t *testing.T) {
	got := MatcherFor("Edit", map[string]any{"file_path": "/foo/.env"})
	if got != "Edit(/foo/.env)" {
		t.Errorf("Edit matcher: got %q", got)
	}
}

func TestMatcherFor_Write(t *testing.T) {
	got := MatcherFor("Write", map[string]any{"file_path": "/foo/bar.go"})
	if got != "Write(/foo/bar.go)" {
		t.Errorf("Write matcher: got %q", got)
	}
}

func TestMatcherFor_UnknownToolHashes(t *testing.T) {
	got := MatcherFor("Skill", map[string]any{"command": "deploy"})
	if !strings.HasPrefix(got, "Skill(h_") {
		t.Errorf("Skill matcher: got %q, want Skill(h_...)", got)
	}
}

func TestMatcherFor_NilInputBash(t *testing.T) {
	got := MatcherFor("Bash", nil)
	if got != "Bash(*)" {
		t.Errorf("nil input: got %q, want Bash(*)", got)
	}
}

func TestMatcherFor_EditMissingPath(t *testing.T) {
	got := MatcherFor("Edit", map[string]any{"other": "stuff"})
	if !strings.HasPrefix(got, "Edit(h_") {
		t.Errorf("Edit no file_path: got %q, want Edit(h_...)", got)
	}
}

func TestMatcherFor_UnknownToolNilInput(t *testing.T) {
	got := MatcherFor("Read", nil)
	if !strings.HasPrefix(got, "Read(h_") {
		t.Errorf("Read nil input: got %q, want Read(h_...)", got)
	}
}

func TestMatcherFor_HashIsDeterministic(t *testing.T) {
	a := MatcherFor("Read", map[string]any{"path": "/x"})
	b := MatcherFor("Read", map[string]any{"path": "/x"})
	if a != b {
		t.Errorf("hash matcher not deterministic: %q vs %q", a, b)
	}
}
