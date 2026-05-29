package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquirePID_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pid")
	release, err := acquirePID(path)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("pid file not created: %v", err)
	}
}

func TestAcquirePID_StalePIDIsTakenOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pid")
	if err := os.WriteFile(path, []byte("999999"), 0o600); err != nil {
		t.Fatalf("write stale pid: %v", err)
	}
	release, err := acquirePID(path)
	if err != nil {
		t.Fatalf("acquire over stale: %v", err)
	}
	defer release()
}

func TestAcquirePID_ConflictsWithLivePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pid")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := acquirePID(path); err == nil {
		t.Error("acquire over live pid should fail")
	}
}
