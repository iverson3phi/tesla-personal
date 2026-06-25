# tesla-sentry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary `tesla-sentry` that turns a Tesla's Sentry Mode on/off via the Fleet API, runnable from crontab.

**Architecture:** One binary with subcommands. Setup subcommands (`keygen`, `register`, `login`) run once to enroll a signing key and obtain an OAuth refresh token. Operational subcommands (`on`, `off`, `status`) refresh the access token, wake the car, and send a signed `SetSentryMode` command via Tesla's official `vehicle-command` Go SDK (internet/Fleet API connector). All state lives in `~/.config/tesla-sentry/`.

**Tech Stack:** Go 1.23+, `github.com/teslamotors/vehicle-command` SDK, Go standard library only for everything else (`net/http`, `encoding/json`, `flag`).

## Global Constraints

- Go version floor: **1.23** (required by the SDK `go.mod`).
- SDK module: **`github.com/teslamotors/vehicle-command`** (latest tagged release). Used packages: `pkg/account`, `pkg/vehicle`, `pkg/protocol`, `pkg/protocol/protobuf/universalmessage`, `pkg/cache`.
- Region: **NA**. Constants (verbatim):
  - Fleet API base / audience: `https://fleet-api.prd.na.vn.cloud.tesla.com`
  - OAuth authorize URL: `https://auth.tesla.com/oauth2/v3/authorize`
  - OAuth token URL: `https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token`
- OAuth scopes (space-separated): `openid offline_access vehicle_device_data vehicle_cmds`
- Sentry Mode is an **INFOTAINMENT** domain command; `StartSession(ctx, nil)` (all domains) is acceptable and simplest.
- Config dir: `~/.config/tesla-sentry/`. All written files must be `chmod 0600` (private key, config, token cache).
- Refresh tokens are single-use and rotate: every successful refresh MUST persist the returned `refresh_token` immediately.
- Module path for this project: `tesla-sentry` (local module).
- Config persisted as JSON (stdlib only — deliberate deviation from the spec's "TOML" to avoid a third-party dependency; it is an internal config file, so the format is not user-facing contract).

---

## File Structure

```
go.mod
cmd/tesla-sentry/main.go          # subcommand dispatch + wiring, exit codes, logging
internal/config/config.go         # Config + TokenCache types, dir resolution, load/save (0600)
internal/config/config_test.go
internal/oauth/oauth.go           # authorize URL, code exchange, refresh, client_credentials
internal/oauth/oauth_test.go
internal/keys/keys.go             # P-256 key pair generation (SDK), PEM output
internal/keys/keys_test.go
internal/tesla/partner.go         # POST /api/1/partner_accounts
internal/tesla/partner_test.go
internal/tesla/vehicledata.go     # GET vehicle_data, parse vehicle_state.sentry_mode
internal/tesla/vehicledata_test.go
internal/tesla/command.go         # SDK wrapper: account → vehicle → wake → SetSentryMode
```

`command.go` is thin glue over the SDK (hard to unit-test without a real car); it is exercised by the `status`/`on`/`off` subcommands and manual E2E. Everything else is unit-tested with `httptest` and temp dirs.

---

### Task 1: Project scaffold + config dir resolution

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `config.Dir() (string, error)` — returns `~/.config/tesla-sentry`, creating it with `0700` if missing.
  - `config.Path(name string) (string, error)` — returns `<Dir>/<name>`.

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd /home/allen/Projects/tesla
go mod init tesla-sentry
go mod edit -go=1.23
```

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Dir` / `undefined: Path`.

- [ ] **Step 4: Write minimal implementation**

`internal/config/config.go`:
```go
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/config/
git commit -m "feat: project scaffold and config dir resolution"
```

---

### Task 2: Config and token-cache load/save

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { ClientID, ClientSecret, VIN, Domain, Region string }`
  - `type Token struct { AccessToken, RefreshToken string; ExpiresAt int64 }`
  - `func LoadConfig() (*Config, error)` / `func (c *Config) Save() error` — file `config.json`, mode `0600`.
  - `func LoadToken() (*Token, error)` / `func (t *Token) Save() error` — file `token.json`, mode `0600`.
  - `func (t *Token) Expired(now int64) bool` — true if `now >= ExpiresAt-60`.

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Config` / `undefined: Token`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/config/config.go`:
```go
import "encoding/json" // add to the existing import block

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config and token-cache load/save with 0600 perms"
```

---

### Task 3: OAuth — refresh, code exchange, client_credentials, authorize URL

**Files:**
- Create: `internal/oauth/oauth.go`
- Test: `internal/oauth/oauth_test.go`

**Interfaces:**
- Consumes: nothing (pure HTTP against Tesla, base URLs injectable for tests).
- Produces:
  - `type Endpoints struct { TokenURL, AuthorizeURL, Audience string }`
  - `func NA() Endpoints` — the verbatim NA constants from Global Constraints.
  - `type TokenResponse struct { AccessToken, RefreshToken string; ExpiresIn int64 }`
  - `func (e Endpoints) Refresh(ctx, clientID, refreshToken string) (*TokenResponse, error)`
  - `func (e Endpoints) Exchange(ctx, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error)`
  - `func (e Endpoints) PartnerToken(ctx, clientID, clientSecret, scope string) (*TokenResponse, error)`
  - `func (e Endpoints) AuthorizeURL(clientID, redirectURI, scope, state string) string`
  - Package var `HTTPClient = http.DefaultClient` and a package var for the token URL so tests can point at `httptest`.

- [ ] **Step 1: Write the failing test**

`internal/oauth/oauth_test.go`:
```go
package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRefreshSendsCorrectBody(t *testing.T) {
	var gotBody url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q", ct)
		}
		_ = r.ParseForm()
		gotBody = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","refresh_token":"RT2","expires_in":28800}`))
	}))
	defer srv.Close()

	e := Endpoints{TokenURL: srv.URL, Audience: "https://aud"}
	got, err := e.Refresh(context.Background(), "cid", "RT1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got.AccessToken != "AT" || got.RefreshToken != "RT2" || got.ExpiresIn != 28800 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if gotBody.Get("grant_type") != "refresh_token" {
		t.Errorf("grant_type = %q", gotBody.Get("grant_type"))
	}
	if gotBody.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", gotBody.Get("client_id"))
	}
	if gotBody.Get("refresh_token") != "RT1" {
		t.Errorf("refresh_token = %q", gotBody.Get("refresh_token"))
	}
	if gotBody.Has("client_secret") {
		t.Errorf("refresh must not send client_secret")
	}
}

func TestPartnerTokenSendsClientCredentials(t *testing.T) {
	var gotBody url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotBody = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"PT","expires_in":300}`))
	}))
	defer srv.Close()

	e := Endpoints{TokenURL: srv.URL, Audience: "https://aud"}
	got, err := e.PartnerToken(context.Background(), "cid", "sec", "openid vehicle_cmds")
	if err != nil {
		t.Fatalf("PartnerToken: %v", err)
	}
	if got.AccessToken != "PT" {
		t.Fatalf("token = %q", got.AccessToken)
	}
	if gotBody.Get("grant_type") != "client_credentials" {
		t.Errorf("grant_type = %q", gotBody.Get("grant_type"))
	}
	if gotBody.Get("client_secret") != "sec" {
		t.Errorf("client_secret = %q", gotBody.Get("client_secret"))
	}
	if gotBody.Get("audience") != "https://aud" {
		t.Errorf("audience = %q", gotBody.Get("audience"))
	}
}

func TestRefreshErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"login_required"}`))
	}))
	defer srv.Close()
	e := Endpoints{TokenURL: srv.URL}
	if _, err := e.Refresh(context.Background(), "cid", "RT"); err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestAuthorizeURL(t *testing.T) {
	e := NA()
	u := e.AuthorizeURL("cid", "https://x.pages.dev/callback", "openid offline_access", "xyz")
	if !strings.HasPrefix(u, "https://auth.tesla.com/oauth2/v3/authorize?") {
		t.Fatalf("bad prefix: %s", u)
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "cid" ||
		q.Get("redirect_uri") != "https://x.pages.dev/callback" ||
		q.Get("scope") != "openid offline_access" || q.Get("state") != "xyz" {
		t.Fatalf("bad query: %v", q)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/oauth/`
Expected: FAIL — `undefined: Endpoints` etc.

- [ ] **Step 3: Write minimal implementation**

`internal/oauth/oauth.go`:
```go
// Package oauth performs Tesla Fleet OAuth token operations.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPClient is overridable in tests.
var HTTPClient = http.DefaultClient

// Endpoints holds region-specific OAuth URLs.
type Endpoints struct {
	TokenURL     string
	AuthorizeURL string
	Audience     string
}

// NA returns the North America endpoints (verbatim Global Constraints).
func NA() Endpoints {
	return Endpoints{
		TokenURL:     "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token",
		AuthorizeURL: "https://auth.tesla.com/oauth2/v3/authorize",
		Audience:     "https://fleet-api.prd.na.vn.cloud.tesla.com",
	}
}

// TokenResponse is the subset of the token endpoint reply we use.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (e Endpoints) post(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tr, nil
}

// Refresh exchanges a refresh token for a new access (and rotated refresh) token.
func (e Endpoints) Refresh(ctx context.Context, clientID, refreshToken string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	})
}

// Exchange swaps an authorization code for tokens.
func (e Endpoints) Exchange(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"audience":      {e.Audience},
		"redirect_uri":  {redirectURI},
	})
}

// PartnerToken gets a client_credentials token for partner-account calls.
func (e Endpoints) PartnerToken(ctx context.Context, clientID, clientSecret, scope string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {scope},
		"audience":      {e.Audience},
	})
}

// AuthorizeURL builds the user-facing consent URL.
func (e Endpoints) AuthorizeURL(clientID, redirectURI, scope, state string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {scope},
		"state":         {state},
	}
	return e.AuthorizeURL_base() + "?" + q.Encode()
}

func (e Endpoints) AuthorizeURL_base() string { return e.AuthorizeURL }
```

Note: the field `AuthorizeURL` and method `AuthorizeURL(...)` would collide; rename the field to `AuthorizeBase`. Apply this correction now: in `Endpoints` rename `AuthorizeURL string` → `AuthorizeBase string`, set it in `NA()`, delete `AuthorizeURL_base`, and have `AuthorizeURL(...)` use `e.AuthorizeBase`. Final method:
```go
func (e Endpoints) AuthorizeURL(clientID, redirectURI, scope, state string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {scope},
		"state":         {state},
	}
	return e.AuthorizeBase + "?" + q.Encode()
}
```
And update the test `Endpoints{TokenURL: ...}` literals (they don't set AuthorizeBase, fine) and `NA()` to include `AuthorizeBase: "https://auth.tesla.com/oauth2/v3/authorize"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/oauth/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/
git commit -m "feat: Tesla OAuth refresh, exchange, partner-token, authorize URL"
```

---

### Task 4: Key pair generation

**Files:**
- Create: `internal/keys/keys.go`
- Test: `internal/keys/keys_test.go`

**Interfaces:**
- Consumes: `github.com/teslamotors/vehicle-command/pkg/protocol`.
- Produces:
  - `func Generate(privPath, pubPath string) error` — writes a P-256 private key (SEC1 PEM, `0600`) and its PKIX public key (`-----BEGIN PUBLIC KEY-----`, `0644`). The public PEM is what gets hosted at `.well-known/appspecific/com.tesla.3p.public-key.pem`.

Add the dependency first:
```bash
go get github.com/teslamotors/vehicle-command@latest
```

- [ ] **Step 1: Write the failing test**

`internal/keys/keys_test.go`:
```go
package keys

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

func TestGenerateProducesLoadableP256Pair(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private-key.pem")
	pub := filepath.Join(dir, "public-key.pem")

	if err := Generate(priv, pub); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Private key must be loadable by the SDK (proves curve + format).
	if _, err := protocol.LoadPrivateKey(priv); err != nil {
		t.Fatalf("SDK LoadPrivateKey: %v", err)
	}
	if info, _ := os.Stat(priv); info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 600", info.Mode().Perm())
	}

	// Public key must be a PKIX "PUBLIC KEY" PEM.
	b, err := os.ReadFile(pub)
	if err != nil {
		t.Fatalf("read pub: %v", err)
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatalf("public PEM type = %v, want PUBLIC KEY", block)
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		t.Fatalf("ParsePKIXPublicKey: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/keys/`
Expected: FAIL — `undefined: Generate`.

- [ ] **Step 3: Write minimal implementation**

`internal/keys/keys.go`:
```go
// Package keys generates the P-256 command-signing key pair.
package keys

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Generate writes a P-256 private key (SEC1 PEM, 0600) to privPath and the
// matching PKIX public key (PUBLIC KEY PEM, 0644) to pubPath.
func Generate(privPath, pubPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return err
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal public: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		return err
	}

	// Compile-time assurance the curve is ECDH-compatible (P-256).
	_, _ = priv.PublicKey.ECDH()
	_ = ecdh.P256()
	return nil
}
```

If the unused `ecdh` import trips the build, remove the `ecdh` import and the two `_ =` lines — they are only a sanity nod, not required.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/keys/`
Expected: PASS (verifies the SDK can load the private key).

- [ ] **Step 5: Commit**

```bash
git add internal/keys/ go.mod go.sum
git commit -m "feat: P-256 signing key pair generation"
```

---

### Task 5: Partner account registration

**Files:**
- Create: `internal/tesla/partner.go`
- Test: `internal/tesla/partner_test.go`

**Interfaces:**
- Consumes: `oauth.TokenResponse.AccessToken` (a partner token).
- Produces:
  - Package var `BaseURL = "https://fleet-api.prd.na.vn.cloud.tesla.com"` (overridable in tests).
  - Package var `HTTPClient = http.DefaultClient`.
  - `func RegisterPartner(ctx context.Context, partnerToken, domain string) error` — `POST {BaseURL}/api/1/partner_accounts` with JSON `{"domain": domain}` and `Authorization: Bearer <partnerToken>`.

- [ ] **Step 1: Write the failing test**

`internal/tesla/partner_test.go`:
```go
package tesla

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterPartnerPostsDomain(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"response":{"domain":"x.pages.dev"}}`))
	}))
	defer srv.Close()

	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	if err := RegisterPartner(context.Background(), "PT", "x.pages.dev"); err != nil {
		t.Fatalf("RegisterPartner: %v", err)
	}
	if gotAuth != "Bearer PT" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotPath != "/api/1/partner_accounts" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["domain"] != "x.pages.dev" {
		t.Errorf("body domain = %q", gotBody["domain"])
	}
}

func TestRegisterPartnerErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(412)
		_, _ = w.Write([]byte(`{"error":"public key not found"}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()
	if err := RegisterPartner(context.Background(), "PT", "x.pages.dev"); err == nil {
		t.Fatal("expected error on 412")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tesla/`
Expected: FAIL — `undefined: BaseURL` / `undefined: RegisterPartner`.

- [ ] **Step 3: Write minimal implementation**

`internal/tesla/partner.go`:
```go
// Package tesla wraps Fleet API HTTP calls and the vehicle-command SDK.
package tesla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BaseURL is the NA Fleet API base; overridable in tests.
var BaseURL = "https://fleet-api.prd.na.vn.cloud.tesla.com"

// HTTPClient is overridable in tests.
var HTTPClient = http.DefaultClient

// RegisterPartner registers the hosting domain with Tesla so vehicles can
// pair the public key. Authorized by a client_credentials partner token.
func RegisterPartner(ctx context.Context, partnerToken, domain string) error {
	body, _ := json.Marshal(map[string]string{"domain": domain})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/api/1/partner_accounts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+partnerToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("partner_accounts %s: %s", resp.Status, string(rb))
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tesla/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tesla/partner.go internal/tesla/partner_test.go
git commit -m "feat: partner account registration"
```

---

### Task 6: Read sentry state (vehicle_data)

**Files:**
- Create: `internal/tesla/vehicledata.go`
- Test: `internal/tesla/vehicledata_test.go`

**Interfaces:**
- Consumes: `BaseURL`, `HTTPClient` (from Task 5).
- Produces:
  - `func SentryState(ctx context.Context, accessToken, vin string) (bool, error)` — `GET {BaseURL}/api/1/vehicles/{vin}/vehicle_data?endpoints=vehicle_state`, returns `response.vehicle_state.sentry_mode`.

- [ ] **Step 1: Write the failing test**

`internal/tesla/vehicledata_test.go`:
```go
package tesla

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSentryStateParsesField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/1/vehicles/VIN123/vehicle_data") {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer AT" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"response":{"vehicle_state":{"sentry_mode":true}}}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	on, err := SentryState(context.Background(), "AT", "VIN123")
	if err != nil {
		t.Fatalf("SentryState: %v", err)
	}
	if !on {
		t.Fatal("expected sentry_mode true")
	}
}

func TestSentryStateErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(408)
		_, _ = w.Write([]byte(`{"error":"vehicle offline"}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()
	if _, err := SentryState(context.Background(), "AT", "VIN123"); err == nil {
		t.Fatal("expected error on 408")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tesla/ -run SentryState`
Expected: FAIL — `undefined: SentryState`.

- [ ] **Step 3: Write minimal implementation**

`internal/tesla/vehicledata.go`:
```go
package tesla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SentryState returns whether Sentry Mode is currently on. The vehicle must be
// online (wake first) for fresh data.
func SentryState(ctx context.Context, accessToken, vin string) (bool, error) {
	url := fmt.Sprintf("%s/api/1/vehicles/%s/vehicle_data?endpoints=vehicle_state", BaseURL, vin)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("vehicle_data %s: %s", resp.Status, string(body))
	}
	var out struct {
		Response struct {
			VehicleState struct {
				SentryMode bool `json:"sentry_mode"`
			} `json:"vehicle_state"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("decode vehicle_data: %w", err)
	}
	return out.Response.VehicleState.SentryMode, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tesla/`
Expected: PASS (all partner + sentry tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tesla/vehicledata.go internal/tesla/vehicledata_test.go
git commit -m "feat: read Sentry Mode state via vehicle_data"
```

---

### Task 7: SDK command wrapper (wake + SetSentryMode)

**Files:**
- Create: `internal/tesla/command.go`

**Interfaces:**
- Consumes: `github.com/teslamotors/vehicle-command/pkg/{account,protocol}`, `pkg/protocol/protobuf/universalmessage`.
- Produces:
  - `func SetSentry(ctx context.Context, accessToken, vin, privateKeyPath string, on bool) error` — loads the key, builds the account, gets the vehicle, wakes it, connects, starts a session, and calls `SetSentryMode`.

This task is thin SDK glue with no unit test (requires a real vehicle). It is verified by `go vet`/`go build` here and by manual E2E in Task 9's status/on/off run.

- [ ] **Step 1: Write the implementation**

`internal/tesla/command.go`:
```go
package tesla

import (
	"context"
	"fmt"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

const userAgent = "tesla-sentry/1.0"

// SetSentry wakes the vehicle and sets Sentry Mode on/off via a signed command.
func SetSentry(ctx context.Context, accessToken, vin, privateKeyPath string, on bool) error {
	key, err := protocol.LoadPrivateKey(privateKeyPath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	acct, err := account.New(accessToken, userAgent)
	if err != nil {
		return fmt.Errorf("build account: %w", err)
	}

	car, err := acct.GetVehicle(ctx, vin, key, nil)
	if err != nil {
		return fmt.Errorf("get vehicle: %w", err)
	}
	defer car.Disconnect()

	// Wakeup blocks until the car reports "online" (or ctx expires).
	if err := car.Wakeup(ctx); err != nil {
		return fmt.Errorf("wake vehicle: %w", err)
	}
	if err := car.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	// nil domains = all subsystems; Sentry Mode routes to INFOTAINMENT.
	if err := car.StartSession(ctx, nil); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	if err := car.SetSentryMode(ctx, on); err != nil {
		return fmt.Errorf("set sentry mode: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles and vets**

Run: `go build ./... && go vet ./...`
Expected: no output (success). If `account.GetVehicle` / `car.Wakeup` / `car.StartSession` signatures differ in the pinned SDK version, adjust the calls to match `go doc github.com/teslamotors/vehicle-command/pkg/account` and `go doc github.com/teslamotors/vehicle-command/pkg/vehicle` (these were verified against the default branch; pin matters).

- [ ] **Step 3: Commit**

```bash
git add internal/tesla/command.go
git commit -m "feat: signed SetSentryMode command via SDK"
```

---

### Task 8: Token manager (refresh-or-load with rotation persistence)

**Files:**
- Create: `internal/tesla/token.go`
- Test: `internal/tesla/token_test.go`

**Interfaces:**
- Consumes: `config.Token`, `config.Config`, `oauth.Endpoints`.
- Produces:
  - `func ValidAccessToken(ctx context.Context, e oauth.Endpoints, cfg *config.Config, tok *config.Token, now int64, save func(*config.Token) error) (string, error)` — returns the cached access token if not expired; otherwise refreshes, persists the rotated token via `save`, and returns the new access token.

Injecting `now` and `save` keeps this unit-testable without touching disk or the clock.

- [ ] **Step 1: Write the failing test**

`internal/tesla/token_test.go`:
```go
package tesla

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tesla-sentry/internal/config"
	"tesla-sentry/internal/oauth"
)

func TestValidAccessTokenReturnsCachedWhenFresh(t *testing.T) {
	cfg := &config.Config{ClientID: "cid"}
	tok := &config.Token{AccessToken: "FRESH", RefreshToken: "RT", ExpiresAt: 10_000}
	saved := false
	at, err := ValidAccessToken(context.Background(), oauth.NA(), cfg, tok, 100, func(*config.Token) error { saved = true; return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if at != "FRESH" {
		t.Fatalf("token = %q, want FRESH", at)
	}
	if saved {
		t.Fatal("should not save when token is fresh")
	}
}

func TestValidAccessTokenRefreshesAndPersistsRotation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"NEW","refresh_token":"RT2","expires_in":28800}`))
	}))
	defer srv.Close()
	oauth.HTTPClient = srv.Client()
	defer func() { oauth.HTTPClient = http.DefaultClient }()

	e := oauth.Endpoints{TokenURL: srv.URL, Audience: "https://aud"}
	cfg := &config.Config{ClientID: "cid"}
	tok := &config.Token{AccessToken: "OLD", RefreshToken: "RT1", ExpiresAt: 50}

	var savedTok *config.Token
	at, err := ValidAccessToken(context.Background(), e, cfg, tok, 100, func(nt *config.Token) error { savedTok = nt; return nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if at != "NEW" {
		t.Fatalf("token = %q, want NEW", at)
	}
	if savedTok == nil || savedTok.RefreshToken != "RT2" || savedTok.AccessToken != "NEW" {
		t.Fatalf("rotation not persisted: %+v", savedTok)
	}
	if savedTok.ExpiresAt != 100+28800 {
		t.Fatalf("ExpiresAt = %d, want %d", savedTok.ExpiresAt, 100+28800)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tesla/ -run ValidAccessToken`
Expected: FAIL — `undefined: ValidAccessToken`.

- [ ] **Step 3: Write minimal implementation**

`internal/tesla/token.go`:
```go
package tesla

import (
	"context"

	"tesla-sentry/internal/config"
	"tesla-sentry/internal/oauth"
)

// ValidAccessToken returns a non-expired access token, refreshing and
// persisting the rotated refresh token when needed.
func ValidAccessToken(ctx context.Context, e oauth.Endpoints, cfg *config.Config, tok *config.Token, now int64, save func(*config.Token) error) (string, error) {
	if tok.AccessToken != "" && !tok.Expired(now) {
		return tok.AccessToken, nil
	}
	tr, err := e.Refresh(ctx, cfg.ClientID, tok.RefreshToken)
	if err != nil {
		return "", err
	}
	newTok := &config.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    now + tr.ExpiresIn,
	}
	if newTok.RefreshToken == "" { // some responses omit a new refresh token
		newTok.RefreshToken = tok.RefreshToken
	}
	if err := save(newTok); err != nil {
		return "", err
	}
	return newTok.AccessToken, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tesla/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tesla/token.go internal/tesla/token_test.go
git commit -m "feat: access-token manager with refresh-token rotation"
```

---

### Task 9: CLI entrypoint — subcommand dispatch and wiring

**Files:**
- Create: `cmd/tesla-sentry/main.go`

**Interfaces:**
- Consumes: all packages above.
- Produces: the `tesla-sentry` binary with subcommands `keygen`, `register`, `login`, `on`, `off`, `status`. Exit 0 on success, non-zero on failure (for cron monitoring).

- [ ] **Step 1: Write the implementation**

`cmd/tesla-sentry/main.go`:
```go
// Command tesla-sentry toggles Tesla Sentry Mode via the Fleet API.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"tesla-sentry/internal/config"
	"tesla-sentry/internal/keys"
	"tesla-sentry/internal/oauth"
	"tesla-sentry/internal/tesla"
)

const commandTimeout = 3 * time.Minute // wake can take a while

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tesla-sentry <keygen|register|login|on|off|status>")
}

func run(cmd string, args []string) error {
	switch cmd {
	case "keygen":
		return cmdKeygen()
	case "register":
		return cmdRegister()
	case "login":
		return cmdLogin()
	case "on":
		return cmdSet(true)
	case "off":
		return cmdSet(false)
	case "status":
		return cmdStatus()
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdKeygen() error {
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	pub, err := config.Path("public-key.pem")
	if err != nil {
		return err
	}
	if err := keys.Generate(priv, pub); err != nil {
		return err
	}
	fmt.Printf("Private key: %s\nPublic key:  %s\n", priv, pub)
	fmt.Println("Host the PUBLIC key at: https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem")
	return nil
}

func cmdRegister() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config (run setup first): %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := oauth.NA()
	pt, err := e.PartnerToken(ctx, cfg.ClientID, cfg.ClientSecret, "openid offline_access vehicle_device_data vehicle_cmds")
	if err != nil {
		return fmt.Errorf("partner token: %w", err)
	}
	if err := tesla.RegisterPartner(ctx, pt.AccessToken, cfg.Domain); err != nil {
		return err
	}
	fmt.Printf("Registered domain %s with Tesla.\n", cfg.Domain)
	return nil
}

func cmdLogin() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config (run setup first): %w", err)
	}
	e := oauth.NA()
	redirect := "https://" + cfg.Domain + "/callback"
	authURL := e.AuthorizeURL(cfg.ClientID, redirect, "openid offline_access vehicle_device_data vehicle_cmds", "tesla-sentry")
	fmt.Println("1. Open this URL in a browser and approve:")
	fmt.Println("   " + authURL)
	fmt.Println("2. After redirect, copy the `code` query parameter from the URL bar.")
	fmt.Print("Paste code: ")
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("read code: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tr, err := e.Exchange(ctx, cfg.ClientID, cfg.ClientSecret, code, redirect)
	if err != nil {
		return err
	}
	tok := &config.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tr.ExpiresIn,
	}
	if err := tok.Save(); err != nil {
		return err
	}
	fmt.Println("Login complete. Refresh token saved.")
	fmt.Println("Final setup step: on your phone, open https://tesla.com/_ak/" + cfg.Domain + " and add the virtual key in the Tesla app.")
	return nil
}

// loadForCommand returns config + a fresh access token, persisting any rotation.
func loadForCommand(ctx context.Context) (*config.Config, string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load config: %w", err)
	}
	tok, err := config.LoadToken()
	if err != nil {
		return nil, "", fmt.Errorf("load token (run `login` first): %w", err)
	}
	at, err := tesla.ValidAccessToken(ctx, oauth.NA(), cfg, tok, time.Now().Unix(), func(nt *config.Token) error { return nt.Save() })
	if err != nil {
		return nil, "", fmt.Errorf("refresh token: %w", err)
	}
	return cfg, at, nil
}

func cmdSet(on bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	if err := tesla.SetSentry(ctx, at, cfg.VIN, priv, on); err != nil {
		return err
	}
	state := "off"
	if on {
		state = "on"
	}
	log.Printf("sentry mode set to %s", state)
	return nil
}

func cmdStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	on, err := tesla.SentryState(ctx, at, cfg.VIN)
	if err != nil {
		return err
	}
	fmt.Printf("sentry mode: %v\n", on)
	return nil
}
```

- [ ] **Step 2: Build the binary and verify subcommand dispatch**

Run:
```bash
go build -o /tmp/tesla-sentry ./cmd/tesla-sentry
/tmp/tesla-sentry 2>&1; echo "exit=$?"
```
Expected: prints usage, `exit=2`.

Run:
```bash
/tmp/tesla-sentry bogus 2>&1; echo "exit=$?"
```
Expected: usage + `unknown command "bogus"`, `exit=1`.

- [ ] **Step 3: Verify keygen works end-to-end (no network)**

Run:
```bash
XDG_CONFIG_HOME=$(mktemp -d) /tmp/tesla-sentry keygen; echo "exit=$?"
```
Expected: prints the two key paths and the well-known hosting hint, `exit=0`.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/tesla-sentry/
git commit -m "feat: CLI entrypoint with keygen/register/login/on/off/status"
```

---

### Task 10: README with one-time setup runbook + crontab

**Files:**
- Create: `README.md`

**Interfaces:** none (documentation).

- [ ] **Step 1: Write the README**

`README.md` must contain, in order:

1. **Prerequisites:** Go 1.23+, a Tesla account, a Cloudflare Pages site.
2. **Build:** `go build -o tesla-sentry ./cmd/tesla-sentry` then `sudo install tesla-sentry /usr/local/bin/`.
3. **One-time setup**, exact sequence:
   - `tesla-sentry keygen`
   - Deploy `~/.config/tesla-sentry/public-key.pem` to Cloudflare Pages at `/.well-known/appspecific/com.tesla.3p.public-key.pem`; note the `xxx.pages.dev` domain.
   - Create app at developer.tesla.com (Allowed Origin `https://xxx.pages.dev`, redirect `https://xxx.pages.dev/callback`, scopes `vehicle_device_data vehicle_cmds`); copy client_id/secret.
   - Write `~/.config/tesla-sentry/config.json` (show the exact JSON shape with `client_id`, `client_secret`, `vin`, `domain` = `xxx.pages.dev`, `region` = `na`); `chmod 600`.
   - `tesla-sentry register`
   - `tesla-sentry login` (paste code).
   - Phone: open `https://tesla.com/_ak/xxx.pages.dev`, add virtual key in Tesla app.
   - `tesla-sentry status` to verify the whole chain.
4. **Crontab** (verbatim):
   ```cron
   0 22 * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
   0 7  * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
   ```
5. **Troubleshooting:** token expiry (re-run `login`), `vehicle offline` (wake retries up to the 3-minute timeout), refresh tokens rotate (don't reuse old ones).

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: setup runbook and crontab usage"
```

---

## Self-Review Notes

- **Spec coverage:** keygen (T4), Cloudflare hosting (README T10), developer app (README), partner register (T5), login/refresh (T3,T8,T9), virtual key enrollment (README), on/off/status (T9), wake+signed command (T7), config/token 0600 (T1,T2), logging/exit codes (T9), retry via SDK wake polling + ctx timeout (T7,T9), tests (T1-T8). All spec sections map to a task.
- **Naming fix locked in:** the `oauth.Endpoints` field/method collision is resolved by renaming the field to `AuthorizeBase` (Task 3, Step 3 correction). All references use `AuthorizeBase` (field) and `AuthorizeURL(...)` (method).
- **Type consistency:** `config.Token`/`config.Config` shapes are identical across T2, T8, T9. `BaseURL`/`HTTPClient` package vars in `internal/tesla` are shared by T5, T6. `ValidAccessToken`'s `save func(*config.Token) error` matches the closure passed in T9.
- **SDK signatures** verified against the SDK default branch; Task 7 Step 2 instructs adjusting to `go doc` output if the pinned tag differs.
