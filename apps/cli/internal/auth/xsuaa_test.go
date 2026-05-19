package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXSUAATokenFetch(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.URL.Path != "/oauth/token" {
			t.Errorf("want /oauth/token, got %s", r.URL.Path)
		}
		if u, p, ok := r.BasicAuth(); !ok || u != "cid" || p != "csecret" {
			t.Errorf("bad basic auth: u=%q p=%q ok=%v", u, p, ok)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "client_credentials" {
			t.Errorf("grant_type=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "jwt-abc",
			"token_type":   "bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	p := NewXSUAA("cid", "csecret", srv.URL+"/oauth/token")
	req, _ := http.NewRequest(http.MethodGet, "https://x.example/api", nil)
	if err := p.Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer jwt-abc" {
		t.Fatalf("Authorization=%q", got)
	}

	// Second Apply should reuse cached token (no second network call).
	if err := p.Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply2: %v", err)
	}
	if calls != 1 {
		t.Fatalf("token endpoint called %d times, want 1 (cache miss)", calls)
	}
}

func TestXSUAAEmptyCreds(t *testing.T) {
	p := NewXSUAA("", "", "")
	req, _ := http.NewRequest(http.MethodGet, "https://x", nil)
	err := p.Apply(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "credentials missing") {
		t.Fatalf("want creds-missing error, got %v", err)
	}
}

func TestXSUAA401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := NewXSUAA("cid", "wrong", srv.URL+"/oauth/token")
	req, _ := http.NewRequest(http.MethodGet, "https://x", nil)
	err := p.Apply(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error on 401")
	}
}
