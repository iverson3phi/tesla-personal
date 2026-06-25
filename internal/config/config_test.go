package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirCreatesUnderXDGConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "") // force HOME-based fallback

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	want := filepath.Join(tmp, ".config", "tesla-sentry")
	if dir != want {
		t.Fatalf("Dir() = %q, want %q", dir, want)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat created dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory at %s", dir)
	}
}

func TestPathJoinsName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := Path("token.json")
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	want := filepath.Join(tmp, ".config", "tesla-sentry", "token.json")
	if p != want {
		t.Fatalf("Path() = %q, want %q", p, want)
	}
}
