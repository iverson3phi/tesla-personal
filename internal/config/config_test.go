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

func TestConfigSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	in := &Config{ClientID: "cid", ClientSecret: "secret", VIN: "5YJ3", Domain: "x.pages.dev", Region: "na"}
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := Path("config.json")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config.json mode = %o, want 600", info.Mode().Perm())
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if *got != *in {
		t.Fatalf("round trip mismatch: %+v vs %+v", got, in)
	}
}

func TestConfigStateFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c := &Config{
		ClientID: "id", ClientSecret: "sec", VIN: "VIN", Domain: "d", Region: "na",
		SentryStateURL: "https://w.example/api/sentry-state", SentryStateToken: "tok",
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.SentryStateURL != c.SentryStateURL || got.SentryStateToken != c.SentryStateToken {
		t.Fatalf("state fields not round-tripped: %+v", got)
	}
}

func TestTokenExpired(t *testing.T) {
	tok := &Token{ExpiresAt: 1000}
	if !tok.Expired(1000) {
		t.Fatalf("expected expired at exactly ExpiresAt")
	}
	if !tok.Expired(950) { // within 60s skew window
		t.Fatalf("expected expired within skew window")
	}
	if tok.Expired(900) {
		t.Fatalf("did not expect expired well before ExpiresAt")
	}
}
