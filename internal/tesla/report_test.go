package tesla

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportSentryStatePutsBody(t *testing.T) {
	var gotMethod, gotAuth, gotState, gotSource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		var body struct{ State, Source string }
		_ = json.Unmarshal(b, &body)
		gotState, gotSource = body.State, body.Source
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := ReportSentryState(context.Background(), srv.URL, "TOK", "on", "command"); err != nil {
		t.Fatalf("ReportSentryState: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotAuth != "Bearer TOK" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotState != "on" || gotSource != "command" {
		t.Errorf("body state/source = %q/%q", gotState, gotSource)
	}
}

func TestReportSentryStateErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()
	if err := ReportSentryState(context.Background(), srv.URL, "BAD", "off", "status"); err == nil {
		t.Fatal("expected error on 401")
	}
}
