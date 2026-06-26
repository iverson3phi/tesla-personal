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
