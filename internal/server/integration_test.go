package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kanywst/approval-hub/internal/config"
	"github.com/kanywst/approval-hub/internal/server"
	"github.com/kanywst/approval-hub/internal/store"
)

// TestIntegration_DecideRoundTripsThroughRealServer spins up the broker over
// a real net.Listener and exercises POST /decide end-to-end including bearer
// middleware, JSON encoding/decoding, and the timeout path. Catches anything
// that the in-process httptest.NewRecorder might hide (header order,
// streaming, etc.).
func TestIntegration_DecideRoundTripsThroughRealServer(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Token = "ahub_integration"
	cfg.UITimeoutSeconds = 1
	st, err := store.Open(filepath.Join(dir, "rules.jsonl"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	srv := server.NewServer(cfg, filepath.Join(dir, "config.json"), st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "ls"},
	})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/decide",
		bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer ahub_integration")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var body struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.HookSpecificOutput.PermissionDecision != "defer" {
		t.Errorf("decision: got %q, want defer (no cache, no resolver)",
			body.HookSpecificOutput.PermissionDecision)
	}
}
