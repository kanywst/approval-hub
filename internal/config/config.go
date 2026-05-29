// Package config loads and persists approval-hub's runtime configuration.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds runtime parameters loaded from config.json.
type Config struct {
	Port               int    `json:"port"`
	Token              string `json:"token"`
	UITimeoutSeconds   int    `json:"ui_timeout_seconds"`
	MaxPendingRequests int    `json:"max_pending_requests"`
	RateLimitPerSecond int    `json:"rate_limit_per_second"`
}

// Defaults returns a Config with built-in defaults. Token is empty.
func Defaults() Config {
	return Config{
		Port:               17456,
		Token:              "",
		UITimeoutSeconds:   60,
		MaxPendingRequests: 100,
		RateLimitPerSecond: 50,
	}
}

// Load reads path. Returns an error wrapping fs.ErrNotExist when path is
// missing so callers can distinguish first-run from corruption.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	c := Defaults()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if c.Token == "" {
		return Config{}, fmt.Errorf("config %s missing token", path)
	}
	return c, nil
}

// Init generates a fresh Config with a new token at path. Refuses to overwrite
// an existing file so install flows cannot silently rotate credentials.
func Init(path string) (Config, error) {
	if _, err := os.Stat(path); err == nil {
		return Config{}, fmt.Errorf("config already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}
	tok, err := generateToken()
	if err != nil {
		return Config{}, err
	}
	c := Defaults()
	c.Token = tok
	if err := Save(path, c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// LoadOrInit returns the existing config if present, otherwise initializes one.
func LoadOrInit(path string) (Config, error) {
	c, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return Init(path)
	}
	return c, err
}

// Save writes c to path with 0600 permission, creating parent dirs as needed.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func generateToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "ahub_" + hex.EncodeToString(b[:]), nil
}

// GenerateToken produces a new "ahub_<64hex>" token using crypto/rand.
// Exposed so other packages (e.g. server) can rotate tokens.
func GenerateToken() (string, error) {
	return generateToken()
}
