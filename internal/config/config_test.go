package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_GeneratesTokenAndWrites0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	c, err := Init(path)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !strings.HasPrefix(c.Token, "ahub_") {
		t.Errorf("token prefix: got %q, want ahub_*", c.Token)
	}
	if want := len("ahub_") + 64; len(c.Token) != want {
		t.Errorf("token length: got %d, want %d", len(c.Token), want)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Errorf("file perm: got %o, want 0600", got)
	}
}

func TestInit_RefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if _, err := Init(path); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	if _, err := Init(path); err == nil {
		t.Error("Init overwrote existing config")
	}
}

func TestLoad_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want, err := Init(path)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestLoad_NotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	_, err := Load(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load missing file: got %v, want fs.ErrNotExist", err)
	}
}

func TestLoad_RejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load did not reject invalid JSON")
	}
}

func TestLoad_RejectsMissingToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(map[string]any{"port": 17456})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load did not reject missing token")
	}
}

func TestLoadOrInit_CreatesIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if c.Token == "" {
		t.Error("LoadOrInit did not generate token")
	}
}

func TestLoadOrInit_ReturnsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	a, err := Init(path)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	b, err := LoadOrInit(path)
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	if a != b {
		t.Errorf("LoadOrInit mismatch:\n a=%+v\n b=%+v", a, b)
	}
}
