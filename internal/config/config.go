// Package config resolves and persists tesla-sentry's on-disk state.
package config

import (
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
