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
