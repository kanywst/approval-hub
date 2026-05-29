package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDir_UsesEnvOverride(t *testing.T) {
	t.Setenv(envDataDir, "/tmp/x")
	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if got != "/tmp/x" {
		t.Errorf("got %q, want /tmp/x", got)
	}
}

func TestDataDir_Default(t *testing.T) {
	t.Setenv(envDataDir, "")
	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if !strings.HasSuffix(got, "approval-hub") {
		t.Errorf("default dataDir should end with approval-hub, got %q", got)
	}
}

func TestPaths_FilenamesUnderDataDir(t *testing.T) {
	t.Setenv(envDataDir, "/data")
	cfg, _ := configPath()
	st, _ := storePath()
	pid, _ := pidPath()
	cases := map[string]string{
		cfg: "/data/config.json",
		st:  "/data/learned-rules.jsonl",
		pid: "/data/approval-hub.pid",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestPaths_AreSiblings(t *testing.T) {
	t.Setenv(envDataDir, "/d")
	cfg, _ := configPath()
	st, _ := storePath()
	if filepath.Dir(cfg) != filepath.Dir(st) {
		t.Errorf("config and store should share dir; cfg=%s store=%s", cfg, st)
	}
}
