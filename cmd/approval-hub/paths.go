package main

import (
	"os"
	"path/filepath"
)

const envDataDir = "APPROVAL_HUB_DATA"

// dataDir returns the directory holding config.json, the rule store, and the
// PID file. APPROVAL_HUB_DATA overrides the OS default location.
func dataDir() (string, error) {
	if d := os.Getenv(envDataDir); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "approval-hub"), nil
}

func configPath() (string, error) {
	d, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func storePath() (string, error) {
	d, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "learned-rules.jsonl"), nil
}

func pidPath() (string, error) {
	d, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "approval-hub.pid"), nil
}
