package sap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("APIKey") != "k" {
			t.Errorf("missing APIKey header")
		}
		if r.URL.Path != "/svc" {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[{"ID":"X1","Title":"T"}]}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, auth.NewAPIKey("k"))

	var got struct {
		D struct {
			Results []struct {
				ID, Title string
			} `json:"results"`
		} `json:"d"`
	}
	if err := c.Get(context.Background(), "/svc", nil, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.D.Results) != 1 || got.D.Results[0].ID != "X1" {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestGetExitCodes(t *testing.T) {
	cases := map[int]int{
		http.StatusUnauthorized: errs.ExitAuth,
		http.StatusNotFound:     errs.ExitNotFound,
		http.StatusConflict:     errs.ExitConflict,
	}
	for status, wantExit := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "x", status)
		}))
		c := New(srv.URL, auth.NewAPIKey("k"))
		c.MaxRetries = 0
		err := c.Get(context.Background(), "/x", nil, nil)
		srv.Close()
		if got := errs.CodeOf(err); got != wantExit {
			t.Errorf("status %d -> exit %d, want %d (err=%v)", status, got, wantExit, err)
		}
	}
}

func TestGetRetryOn503(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[]}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, auth.NewAPIKey("k"))
	c.MaxRetries = 5

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var out struct{}
	if err := c.Get(ctx, "/x", nil, &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if atomic.LoadInt32(&hits) != 3 {
		t.Fatalf("hits=%d, want 3", hits)
	}
}

func TestGetAuthErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := New(srv.URL, auth.NewAPIKey("")) // empty key -> Apply errors
	c.MaxRetries = 0
	err := c.Get(context.Background(), "/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "API key is empty") {
		t.Fatalf("want empty-key error, got %v", err)
	}
}
