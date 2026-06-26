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
