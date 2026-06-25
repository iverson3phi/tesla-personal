// Package config resolves and persists tesla-sentry's on-disk state.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appDir = "tesla-sentry"

// Dir returns the tesla-sentry config directory, creating it (0700) if absent.
// Honors XDG_CONFIG_HOME, falling back to $HOME/.config.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns <Dir>/<name>, creating the directory if needed.
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Config holds static, user-provided settings.
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	VIN          string `json:"vin"`
	Domain       string `json:"domain"`
	Region       string `json:"region"`
}

// Token is the cached OAuth state; refresh rotates RefreshToken.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // unix seconds
}

func writeJSON(name string, v any) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func readJSON(name string, v any) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (c *Config) Save() error    { return writeJSON("config.json", c) }
func LoadConfig() (*Config, error) {
	var c Config
	if err := readJSON("config.json", &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (t *Token) Save() error { return writeJSON("token.json", t) }
func LoadToken() (*Token, error) {
	var t Token
	if err := readJSON("token.json", &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Expired reports whether the access token is at/near expiry (60s skew).
func (t *Token) Expired(now int64) bool { return now >= t.ExpiresAt-60 }
