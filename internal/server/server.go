// Package server implements approval-hub's HTTP broker for PreToolUse hooks.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kanywst/approval-hub/internal/config"
	"github.com/kanywst/approval-hub/internal/engine"
	"github.com/kanywst/approval-hub/internal/store"
)

// Pending is a request waiting for a TUI client decision.
type Pending struct {
	ID         string
	SessionID  string
	Cwd        string
	ToolName   string
	ToolInput  map[string]any
	Matcher    string
	ReceivedAt time.Time
	Resolver   chan Decision
}

// Decision is a TUI-supplied resolution for a Pending.
type Decision struct {
	PermissionDecision       string
	PermissionDecisionReason string
}

// PendingQueue holds in-flight pending requests, bounded by max.
type PendingQueue struct {
	max     int
	mu      sync.Mutex
	pending map[string]*Pending
}

func newPendingQueue(max int) *PendingQueue {
	return &PendingQueue{max: max, pending: make(map[string]*Pending)}
}

// Add registers p, returning an error if the queue is full.
func (q *PendingQueue) Add(p *Pending) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) >= q.max {
		return errors.New("pending queue full")
	}
	q.pending[p.ID] = p
	return nil
}

// Remove deletes the entry; safe after resolution or timeout.
func (q *PendingQueue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.pending, id)
}

// Resolve sends dec to the pending's Resolver channel.
func (q *PendingQueue) Resolve(id string, dec Decision) error {
	q.mu.Lock()
	p, ok := q.pending[id]
	q.mu.Unlock()
	if !ok {
		return fmt.Errorf("pending not found: %s", id)
	}
	select {
	case p.Resolver <- dec:
		return nil
	default:
		return errors.New("resolver already filled")
	}
}

// Snapshot returns the current pending list (copy of pointers).
func (q *PendingQueue) Snapshot() []*Pending {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Pending, 0, len(q.pending))
	for _, p := range q.pending {
		out = append(out, p)
	}
	return out
}

// RateLimiter is a simple per-second token bucket.
type RateLimiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	rate   float64
	last   time.Time
}

func newRateLimiter(perSec int, now time.Time) *RateLimiter {
	maxT := float64(perSec)
	return &RateLimiter{tokens: maxT, max: maxT, rate: maxT, last: now}
}

// Allow consumes one token if available at now, returning whether to proceed.
func (r *RateLimiter) Allow(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rate <= 0 {
		return false
	}
	if elapsed := now.Sub(r.last).Seconds(); elapsed > 0 {
		r.tokens += elapsed * r.rate
		if r.tokens > r.max {
			r.tokens = r.max
		}
		r.last = now
	}
	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Server is the HTTP broker.
type Server struct {
	cfg        config.Config
	configPath string
	store      *store.Store
	queue      *PendingQueue
	rate       *RateLimiter
	bus        *EventBus
	now        func() time.Time
	nextID     func() string
	genToken   func() (string, error)
	cfgMu      sync.RWMutex
}

// NewServer creates a Server with production defaults. configPath is required
// for token rotation; pass "" if rotation is disabled.
func NewServer(cfg config.Config, configPath string, st *store.Store) *Server {
	now := time.Now
	return &Server{
		cfg:        cfg,
		configPath: configPath,
		store:      st,
		queue:      newPendingQueue(cfg.MaxPendingRequests),
		rate:       newRateLimiter(cfg.RateLimitPerSecond, now()),
		bus:        newEventBus(64),
		now:        now,
		nextID:     defaultIDGen,
		genToken:   config.GenerateToken,
	}
}

// Queue returns the underlying pending queue.
func (s *Server) Queue() *PendingQueue { return s.queue }

// Bus returns the event bus.
func (s *Server) Bus() *EventBus { return s.bus }

// Resolve forwards a TUI decision to the pending queue.
func (s *Server) Resolve(id string, dec Decision) error {
	return s.queue.Resolve(id, dec)
}

// Handler returns the bearer-authenticated http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /decide", s.handleDecide)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /pending", s.handlePending)
	mux.HandleFunc("POST /pending/{id}/resolve", s.handleResolve)
	mux.HandleFunc("GET /rules", s.handleRules)
	mux.HandleFunc("DELETE /rules/{id}", s.handleRevoke)
	mux.HandleFunc("POST /rotate-token", s.handleRotateToken)
	return s.authMiddleware(mux)
}

func (s *Server) currentToken() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.Token
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	const prefix = "Bearer "
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, prefix) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := []byte(h[len(prefix):])
		want := []byte(s.currentToken())
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type hookPayload struct {
	SessionID     string         `json:"session_id"`
	Cwd           string         `json:"cwd"`
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
}

type decisionOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

type decisionResponse struct {
	RequestID          string         `json:"request_id"`
	HookSpecificOutput decisionOutput `json:"hookSpecificOutput"`
}

// pendingView is the JSON-safe projection of Pending (omits the Resolver chan).
type pendingView struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	Cwd        string         `json:"cwd"`
	ToolName   string         `json:"tool_name"`
	ToolInput  map[string]any `json:"tool_input"`
	Matcher    string         `json:"matcher"`
	ReceivedAt time.Time      `json:"received_at"`
}

func viewOf(p *Pending) pendingView {
	return pendingView{
		ID:         p.ID,
		SessionID:  p.SessionID,
		Cwd:        p.Cwd,
		ToolName:   p.ToolName,
		ToolInput:  p.ToolInput,
		Matcher:    p.Matcher,
		ReceivedAt: p.ReceivedAt,
	}
}

func (s *Server) handleDecide(w http.ResponseWriter, r *http.Request) {
	if !s.rate.Allow(s.now()) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	var p hookPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if p.ToolName == "" {
		http.Error(w, "missing tool_name", http.StatusBadRequest)
		return
	}

	matcher := engine.MatcherFor(p.ToolName, p.ToolInput)

	if rule, ok := s.store.Lookup(matcher, s.now()); ok {
		s.writeDecision(w, s.nextID(), string(rule.Decision),
			fmt.Sprintf("matched rule %s", matcher))
		return
	}

	id := s.nextID()
	pending := &Pending{
		ID:         id,
		SessionID:  p.SessionID,
		Cwd:        p.Cwd,
		ToolName:   p.ToolName,
		ToolInput:  p.ToolInput,
		Matcher:    matcher,
		ReceivedAt: s.now(),
		Resolver:   make(chan Decision, 1),
	}
	if err := s.queue.Add(pending); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	s.bus.Publish(Event{Type: EventPendingAdded, Data: viewOf(pending)})
	defer s.queue.Remove(id)

	timeout := time.Duration(s.cfg.UITimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	select {
	case dec := <-pending.Resolver:
		s.writeDecision(w, id, dec.PermissionDecision, dec.PermissionDecisionReason)
	case <-ctx.Done():
		s.bus.Publish(Event{
			Type: EventPendingExpired,
			Data: map[string]any{"id": id},
		})
		s.writeDecision(w, id, "defer", "ui timeout")
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ch, unsub := s.bus.Subscribe()
	defer unsub()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handlePending(w http.ResponseWriter, _ *http.Request) {
	ps := s.queue.Snapshot()
	views := make([]pendingView, 0, len(ps))
	for _, p := range ps {
		views = append(views, viewOf(p))
	}
	writeJSON(w, http.StatusOK, views)
}

type resolveBody struct {
	Decision   string `json:"decision"`
	Scope      string `json:"scope"`
	TTLSeconds int    `json:"ttl_seconds"`
	Matcher    string `json:"matcher"`
	Reason     string `json:"reason"`
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body resolveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Decision == "" {
		http.Error(w, "missing decision", http.StatusBadRequest)
		return
	}

	if body.Scope == "persist" || body.Scope == "ttl" {
		if body.Matcher == "" {
			http.Error(w, "missing matcher", http.StatusBadRequest)
			return
		}
		dec := store.DecisionAllow
		if body.Decision == "deny" {
			dec = store.DecisionDeny
		}
		var exp *time.Time
		if body.Scope == "ttl" && body.TTLSeconds > 0 {
			e := s.now().Add(time.Duration(body.TTLSeconds) * time.Second)
			exp = &e
		}
		rule := store.Rule{
			ID:        s.nextID(),
			Rule:      body.Matcher,
			Decision:  dec,
			ExpiresAt: exp,
			AddedAt:   s.now(),
		}
		if err := s.store.AddRule(rule); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		s.bus.Publish(Event{Type: EventRuleAdded, Data: rule})
	}

	if err := s.Resolve(id, Decision{
		PermissionDecision:       body.Decision,
		PermissionDecisionReason: body.Reason,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.bus.Publish(Event{
		Type: EventPendingResolved,
		Data: map[string]any{"id": id, "decision": body.Decision},
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRules(w http.ResponseWriter, _ *http.Request) {
	rules := s.store.List(s.now())
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Revoke(id, s.now()); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.bus.Publish(Event{
		Type: EventRuleRevoked,
		Data: map[string]any{"id": id},
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRotateToken(w http.ResponseWriter, _ *http.Request) {
	newTok, err := s.genToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.cfgMu.Lock()
	s.cfg.Token = newTok
	cfgCopy := s.cfg
	s.cfgMu.Unlock()
	if s.configPath != "" {
		if err := config.Save(s.configPath, cfgCopy); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": newTok})
}

func (s *Server) writeDecision(w http.ResponseWriter, id, decision, reason string) {
	resp := decisionResponse{
		RequestID: id,
		HookSpecificOutput: decisionOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       decision,
			PermissionDecisionReason: reason,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func defaultIDGen() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
