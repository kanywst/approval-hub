package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kanywst/approval-hub/internal/config"
	"github.com/kanywst/approval-hub/internal/store"
)

func newTestServer(t *testing.T, ttlSec int) *Server {
	t.Helper()
	cfg := config.Defaults()
	cfg.Token = "ahub_test"
	cfg.UITimeoutSeconds = ttlSec
	cfg.RateLimitPerSecond = 1000
	cfg.MaxPendingRequests = 5
	st, err := store.Open(filepath.Join(t.TempDir(), "rules.jsonl"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	now := time.Now
	return &Server{
		cfg:        cfg,
		configPath: filepath.Join(t.TempDir(), "config.json"),
		store:      st,
		queue:      newPendingQueue(cfg.MaxPendingRequests),
		rate:       newRateLimiter(cfg.RateLimitPerSecond, now()),
		bus:        newEventBus(64),
		now:        now,
		nextID:     incrIDGen(),
		genToken:   func() (string, error) { return "ahub_rotated", nil },
	}
}

func incrIDGen() func() string {
	var n int64
	return func() string {
		v := atomic.AddInt64(&n, 1)
		return fmt.Sprintf("req%d", v)
	}
}

func authReq(method, path, token string, body []byte) *http.Request {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func do(srv *Server, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func decideReq(srv *Server, payload map[string]any, token string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	return do(srv, authReq(http.MethodPost, "/decide", token, body))
}

func parseDecision(t *testing.T, body io.Reader) decisionResponse {
	t.Helper()
	var r decisionResponse
	if err := json.NewDecoder(body).Decode(&r); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return r
}

func TestDecide_Unauthorized_NoToken(t *testing.T) {
	srv := newTestServer(t, 1)
	rr := decideReq(srv, map[string]any{}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestDecide_Unauthorized_BadToken(t *testing.T) {
	srv := newTestServer(t, 1)
	rr := decideReq(srv, map[string]any{}, "wrong")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestDecide_BadJSON(t *testing.T) {
	srv := newTestServer(t, 1)
	req := authReq(http.MethodPost, "/decide", "ahub_test", []byte("nope"))
	rr := do(srv, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestDecide_MissingToolName(t *testing.T) {
	srv := newTestServer(t, 1)
	rr := decideReq(srv, map[string]any{"session_id": "s1"}, "ahub_test")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestDecide_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t, 1)
	rr := do(srv, authReq(http.MethodGet, "/decide", "ahub_test", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rr.Code)
	}
}

func TestDecide_CacheHitAllow(t *testing.T) {
	srv := newTestServer(t, 1)
	if err := srv.store.AddRule(store.Rule{
		ID: "r1", Rule: "Bash(npm:*)", Decision: store.DecisionAllow,
		AddedAt: time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	rr := decideReq(srv, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "npm test"},
	}, "ahub_test")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	r := parseDecision(t, rr.Body)
	if got := r.HookSpecificOutput.PermissionDecision; got != "allow" {
		t.Errorf("decision: got %q, want allow", got)
	}
}

func TestDecide_CacheHitDeny(t *testing.T) {
	srv := newTestServer(t, 1)
	if err := srv.store.AddRule(store.Rule{
		ID: "r1", Rule: "Bash(rm:*)", Decision: store.DecisionDeny,
		AddedAt: time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	rr := decideReq(srv, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "rm -rf /"},
	}, "ahub_test")
	r := parseDecision(t, rr.Body)
	if got := r.HookSpecificOutput.PermissionDecision; got != "deny" {
		t.Errorf("decision: got %q, want deny", got)
	}
}

func TestDecide_TimeoutReturnsDefer(t *testing.T) {
	srv := newTestServer(t, 1)
	rr := decideReq(srv, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "ls"},
	}, "ahub_test")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	r := parseDecision(t, rr.Body)
	if got := r.HookSpecificOutput.PermissionDecision; got != "defer" {
		t.Errorf("decision: got %q, want defer", got)
	}
}

func TestDecide_ResolverAllow(t *testing.T) {
	srv := newTestServer(t, 30)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "ls"},
		}, "ahub_test")
	}()
	pendID := waitForPending(t, srv)
	if err := srv.Resolve(pendID, Decision{
		PermissionDecision:       "allow",
		PermissionDecisionReason: "user yes",
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	rr := <-done
	r := parseDecision(t, rr.Body)
	if got := r.HookSpecificOutput.PermissionDecision; got != "allow" {
		t.Errorf("decision: got %q, want allow", got)
	}
}

func TestDecide_QueueFullReturns503(t *testing.T) {
	srv := newTestServer(t, 30)
	srv.queue = newPendingQueue(1)
	go func() {
		_ = decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "first"},
		}, "ahub_test")
	}()
	waitForPending(t, srv)
	rr := decideReq(srv, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "second"},
	}, "ahub_test")
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rr.Code)
	}
}

func waitForPending(t *testing.T, srv *Server) string {
	t.Helper()
	for range 200 {
		ps := srv.queue.Snapshot()
		if len(ps) > 0 {
			return ps[0].ID
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("pending never appeared")
	return ""
}

func TestRateLimiter_AllowsBurst(t *testing.T) {
	now := time.Now()
	r := newRateLimiter(5, now)
	for i := range 5 {
		if !r.Allow(now) {
			t.Errorf("burst %d rejected", i)
		}
	}
	if r.Allow(now) {
		t.Error("over-burst not rejected")
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	now := time.Now()
	r := newRateLimiter(10, now)
	for range 10 {
		_ = r.Allow(now)
	}
	if r.Allow(now) {
		t.Error("immediate over-burst not rejected")
	}
	if !r.Allow(now.Add(time.Second)) {
		t.Error("post-refill not allowed")
	}
}

func TestHandlePending_ListsQueueSnapshot(t *testing.T) {
	srv := newTestServer(t, 30)
	go func() {
		_ = decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "ls"},
		}, "ahub_test")
	}()
	waitForPending(t, srv)
	rr := do(srv, authReq(http.MethodGet, "/pending", "ahub_test", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var got []pendingView
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d pending, want 1", len(got))
	}
}

func TestHandleResolve_OnceDoesNotAddRule(t *testing.T) {
	srv := newTestServer(t, 30)
	go func() {
		_ = decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "ls"},
		}, "ahub_test")
	}()
	pendID := waitForPending(t, srv)
	body, _ := json.Marshal(resolveBody{
		Decision: "allow", Scope: "once", Reason: "test",
	})
	rr := do(srv, authReq(http.MethodPost, "/pending/"+pendID+"/resolve", "ahub_test", body))
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rr.Code)
	}
	if got := srv.store.List(time.Now()); len(got) != 0 {
		t.Errorf("rule store should be empty, got %d", len(got))
	}
}

func TestHandleResolve_PersistAddsRule(t *testing.T) {
	srv := newTestServer(t, 30)
	go func() {
		_ = decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "npm test"},
		}, "ahub_test")
	}()
	pendID := waitForPending(t, srv)
	body, _ := json.Marshal(resolveBody{
		Decision: "allow", Scope: "persist", Matcher: "Bash(npm:*)",
	})
	rr := do(srv, authReq(http.MethodPost, "/pending/"+pendID+"/resolve", "ahub_test", body))
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rr.Code)
	}
	rules := srv.store.List(time.Now())
	if len(rules) != 1 || rules[0].Rule != "Bash(npm:*)" {
		t.Errorf("rules: got %+v, want one Bash(npm:*)", rules)
	}
}

func TestHandleRules_ReturnsList(t *testing.T) {
	srv := newTestServer(t, 30)
	if err := srv.store.AddRule(store.Rule{
		ID: "r1", Rule: "Bash(x)", Decision: store.DecisionAllow,
		AddedAt: time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	rr := do(srv, authReq(http.MethodGet, "/rules", "ahub_test", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var rules []store.Rule
	if err := json.NewDecoder(rr.Body).Decode(&rules); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("got %d rules, want 1", len(rules))
	}
}

func TestHandleRevoke_Tombstones(t *testing.T) {
	srv := newTestServer(t, 30)
	if err := srv.store.AddRule(store.Rule{
		ID: "r1", Rule: "Bash(x)", Decision: store.DecisionAllow,
		AddedAt: time.Now(),
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	rr := do(srv, authReq(http.MethodDelete, "/rules/r1", "ahub_test", nil))
	if rr.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rr.Code)
	}
	if _, ok := srv.store.Lookup("Bash(x)", time.Now()); ok {
		t.Error("rule still present after revoke")
	}
}

func TestHandleRotateToken_GeneratesNew(t *testing.T) {
	srv := newTestServer(t, 30)
	rr := do(srv, authReq(http.MethodPost, "/rotate-token", "ahub_test", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["token"] != "ahub_rotated" {
		t.Errorf("token: got %q, want ahub_rotated", body["token"])
	}
	// Old token now rejected, new token works
	old := do(srv, authReq(http.MethodGet, "/rules", "ahub_test", nil))
	if old.Code != http.StatusUnauthorized {
		t.Errorf("old token: got %d, want 401", old.Code)
	}
	fresh := do(srv, authReq(http.MethodGet, "/rules", "ahub_rotated", nil))
	if fresh.Code != http.StatusOK {
		t.Errorf("new token: got %d, want 200", fresh.Code)
	}
}

func TestSSE_ReceivesPendingAdded(t *testing.T) {
	srv := newTestServer(t, 30)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	req, _ := http.NewRequest(http.MethodGet, httpSrv.URL+"/events", nil)
	req.Header.Set("Authorization", "Bearer ahub_test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sse status: got %d", resp.StatusCode)
	}
	// Wait for subscriber to register
	for range 100 {
		if srv.bus.Subscribers() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	go func() {
		_ = decideReq(srv, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": "ls"},
		}, "ahub_test")
	}()

	br := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(3 * time.Second)
	var sawEvent bool
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read sse: %v", err)
		}
		if strings.HasPrefix(line, "event: pending_added") {
			sawEvent = true
			break
		}
	}
	if !sawEvent {
		t.Error("did not receive pending_added event")
	}
}
