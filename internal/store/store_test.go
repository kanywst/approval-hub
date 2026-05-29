package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "rules.jsonl"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestOpen_CreatesEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "rules.jsonl")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := s.List(time.Now()); len(got) != 0 {
		t.Errorf("empty store: got %d rules, want 0", len(got))
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("store file not created: %v", err)
	}
}

func TestAddAndLookup(t *testing.T) {
	s := newStore(t)
	r := Rule{
		ID:       "r1",
		Rule:     "Bash(npm test:*)",
		Decision: DecisionAllow,
		AddedAt:  time.Now(),
	}
	if err := s.AddRule(r); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	got, ok := s.Lookup("Bash(npm test:*)", time.Now())
	if !ok {
		t.Fatal("Lookup miss after AddRule")
	}
	if got.Decision != DecisionAllow {
		t.Errorf("Decision: got %q want allow", got.Decision)
	}
}

func TestPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.jsonl")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if err := s1.AddRule(Rule{
		ID:       "r1",
		Rule:     "Bash(npm test:*)",
		Decision: DecisionAllow,
		AddedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if _, ok := s2.Lookup("Bash(npm test:*)", time.Now()); !ok {
		t.Error("rule did not persist across reopen")
	}
}

func TestRevoke(t *testing.T) {
	s := newStore(t)
	if err := s.AddRule(Rule{
		ID:       "r1",
		Rule:     "Bash(rm *)",
		Decision: DecisionDeny,
		AddedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if err := s.Revoke("r1", time.Now()); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, ok := s.Lookup("Bash(rm *)", time.Now()); ok {
		t.Error("rule still findable after revoke")
	}
}

func TestRevokePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.jsonl")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if err := s1.AddRule(Rule{
		ID: "r1", Rule: "Bash(x)", Decision: DecisionAllow,
		AddedAt: time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if err := s1.Revoke("r1", time.Now()); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if _, ok := s2.Lookup("Bash(x)", time.Now()); ok {
		t.Error("revoke did not persist")
	}
}

func TestRevoke_UnknownID(t *testing.T) {
	s := newStore(t)
	if err := s.Revoke("nope", time.Now()); err == nil {
		t.Error("Revoke of unknown ID did not error")
	}
}

func TestTTL_ExpiredIsMiss(t *testing.T) {
	s := newStore(t)
	past := time.Now().Add(-1 * time.Hour)
	if err := s.AddRule(Rule{
		ID: "r1", Rule: "Bash(x)", Decision: DecisionAllow,
		AddedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: &past,
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if _, ok := s.Lookup("Bash(x)", time.Now()); ok {
		t.Error("expired rule should be miss")
	}
}

func TestTTL_FutureIsHit(t *testing.T) {
	s := newStore(t)
	future := time.Now().Add(1 * time.Hour)
	if err := s.AddRule(Rule{
		ID: "r1", Rule: "Bash(x)", Decision: DecisionAllow,
		AddedAt: time.Now(), ExpiresAt: &future,
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if _, ok := s.Lookup("Bash(x)", time.Now()); !ok {
		t.Error("future-TTL rule should hit")
	}
}

func TestAdd_DuplicateMatcher(t *testing.T) {
	s := newStore(t)
	r := Rule{
		ID: "r1", Rule: "Bash(x)", Decision: DecisionAllow,
		AddedAt: time.Now(),
	}
	if err := s.AddRule(r); err != nil {
		t.Fatalf("first AddRule: %v", err)
	}
	r2 := Rule{
		ID: "r2", Rule: "Bash(x)", Decision: DecisionDeny,
		AddedAt: time.Now(),
	}
	if err := s.AddRule(r2); err == nil {
		t.Error("AddRule did not reject duplicate matcher")
	}
}

func TestList_ExcludesExpired(t *testing.T) {
	s := newStore(t)
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("AddRule: %v", err)
		}
	}
	must(s.AddRule(Rule{
		ID: "live", Rule: "Bash(a)", Decision: DecisionAllow,
		AddedAt: time.Now(), ExpiresAt: &future,
	}))
	must(s.AddRule(Rule{
		ID: "dead", Rule: "Bash(b)", Decision: DecisionAllow,
		AddedAt: time.Now(), ExpiresAt: &past,
	}))
	must(s.AddRule(Rule{
		ID: "forever", Rule: "Bash(c)", Decision: DecisionAllow,
		AddedAt: time.Now(),
	}))
	got := s.List(time.Now())
	if len(got) != 2 {
		t.Errorf("List: got %d, want 2 (live + forever)", len(got))
	}
}
