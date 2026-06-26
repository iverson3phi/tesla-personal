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
	if gotBody.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", gotBody.Get("client_id"))
	}
	if gotBody.Get("client_secret") != "sec" {
		t.Errorf("client_secret = %q", gotBody.Get("client_secret"))
	}
	if gotBody.Get("scope") != "openid vehicle_cmds" {
		t.Errorf("scope = %q", gotBody.Get("scope"))
	}
	if gotBody.Get("audience") != "https://aud" {
		t.Errorf("audience = %q", gotBody.Get("audience"))
	}
}

func TestExchangeSendsCorrectBody(t *testing.T) {
	var gotBody url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q", ct)
		}
		_ = r.ParseForm()
		gotBody = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","refresh_token":"RT","expires_in":300}`))
	}))
	defer srv.Close()

	e := Endpoints{TokenURL: srv.URL, Audience: "https://aud"}
	got, err := e.Exchange(context.Background(), "cid", "sec", "thecode", "https://x.pages.dev/callback")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got.AccessToken != "AT" {
		t.Fatalf("access_token = %q", got.AccessToken)
	}
	if gotBody.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q", gotBody.Get("grant_type"))
	}
	if gotBody.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", gotBody.Get("client_id"))
	}
	if gotBody.Get("client_secret") != "sec" {
		t.Errorf("client_secret = %q", gotBody.Get("client_secret"))
	}
	if gotBody.Get("code") != "thecode" {
		t.Errorf("code = %q", gotBody.Get("code"))
	}
	if gotBody.Get("audience") != "https://aud" {
		t.Errorf("audience = %q", gotBody.Get("audience"))
	}
	if gotBody.Get("redirect_uri") != "https://x.pages.dev/callback" {
		t.Errorf("redirect_uri = %q", gotBody.Get("redirect_uri"))
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
