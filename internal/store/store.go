// Package store persists approval-hub's learned matcher rules as a JSONL log
// and exposes an in-memory index over the live (non-revoked, non-expired) set.
package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Decision is the cached outcome attached to a rule.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Record types in the JSONL log.
const (
	TypeRuleAdded   = "rule_added"
	TypeRuleRevoked = "rule_revoked"
)

// Record is a single line in the JSONL log. Field validity depends on Type.
type Record struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Rule      string     `json:"rule,omitempty"`
	Decision  Decision   `json:"decision,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	AddedAt   time.Time  `json:"added_at,omitzero"`
	SessionID string     `json:"session_id,omitempty"`
	Cwd       string     `json:"cwd,omitempty"`
	TargetID  string     `json:"target_id,omitempty"`
	RevokedAt time.Time  `json:"revoked_at,omitzero"`
}

// Rule is the in-memory view of a live rule.
type Rule struct {
	ID        string
	Rule      string
	Decision  Decision
	ExpiresAt *time.Time
	AddedAt   time.Time
	SessionID string
	Cwd       string
}

// IsLive reports whether the rule is still in effect at now.
func (r Rule) IsLive(now time.Time) bool {
	return r.ExpiresAt == nil || now.Before(*r.ExpiresAt)
}

// Store is an append-only JSONL rule store with an in-memory index.
type Store struct {
	path      string
	mu        sync.RWMutex
	rules     map[string]Rule
	byMatcher map[string]string
}

// Open reads path and reconstructs the in-memory index, creating the file
// (and parent dirs) if they do not yet exist.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	s := &Store{
		path:      path,
		rules:     make(map[string]Rule),
		byMatcher: make(map[string]string),
	}
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("parse store record: %w", err)
		}
		s.apply(r)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan store: %w", err)
	}
	return s, nil
}

// apply mutates the in-memory index for one record. Caller holds the write
// lock or runs single-threaded inside Open.
func (s *Store) apply(r Record) {
	switch r.Type {
	case TypeRuleAdded:
		s.rules[r.ID] = Rule{
			ID:        r.ID,
			Rule:      r.Rule,
			Decision:  r.Decision,
			ExpiresAt: r.ExpiresAt,
			AddedAt:   r.AddedAt,
			SessionID: r.SessionID,
			Cwd:       r.Cwd,
		}
		s.byMatcher[r.Rule] = r.ID
	case TypeRuleRevoked:
		if existing, ok := s.rules[r.TargetID]; ok {
			delete(s.byMatcher, existing.Rule)
			delete(s.rules, r.TargetID)
		}
	}
}

// AddRule appends a rule_added record. Refuses duplicate matchers so Lookup
// stays unambiguous; callers Revoke first to replace.
func (s *Store) AddRule(r Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byMatcher[r.Rule]; exists {
		return fmt.Errorf("matcher already present: %s", r.Rule)
	}
	rec := Record{
		Type:      TypeRuleAdded,
		ID:        r.ID,
		Rule:      r.Rule,
		Decision:  r.Decision,
		ExpiresAt: r.ExpiresAt,
		AddedAt:   r.AddedAt,
		SessionID: r.SessionID,
		Cwd:       r.Cwd,
	}
	if err := s.appendRecord(rec); err != nil {
		return err
	}
	s.apply(rec)
	return nil
}

// Revoke appends a tombstone for the given rule ID.
func (s *Store) Revoke(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	rec := Record{
		Type:      TypeRuleRevoked,
		ID:        fmt.Sprintf("tomb_%d", at.UnixNano()),
		TargetID:  id,
		RevokedAt: at,
	}
	if err := s.appendRecord(rec); err != nil {
		return err
	}
	s.apply(rec)
	return nil
}

// Lookup returns the live rule for the given matcher at now, or false.
func (s *Store) Lookup(matcher string, now time.Time) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byMatcher[matcher]
	if !ok {
		return Rule{}, false
	}
	r := s.rules[id]
	if !r.IsLive(now) {
		return Rule{}, false
	}
	return r, true
}

// List returns all live rules at now.
func (s *Store) List(now time.Time) []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Rule, 0, len(s.rules))
	for _, r := range s.rules {
		if r.IsLive(now) {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) appendRecord(r Record) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open store for append: %w", err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("marshal record: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write record: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close store after append: %w", err)
	}
	return nil
}
