package sap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
)

func TestAuditorRecordsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-abc")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var events []AuditEvent
	var mu sync.Mutex
	c := New(srv.URL, auth.NewAPIKey("k"))
	c.Audit = AuditorFunc(func(ctx context.Context, ev AuditEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	if err := c.Get(context.Background(), "/svc", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Method != "GET" || ev.Status != 200 || ev.RequestID != "req-abc" {
		t.Fatalf("event=%+v", ev)
	}
	if ev.Provider != "apikey" {
		t.Fatalf("provider=%q", ev.Provider)
	}
	if ev.Duration <= 0 {
		t.Fatalf("duration not set")
	}
}

func TestAuditorRecordsRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var events []AuditEvent
	c := New(srv.URL, auth.NewAPIKey("k"))
	c.MaxRetries = 5
	c.Audit = AuditorFunc(func(ctx context.Context, ev AuditEvent) {
		events = append(events, ev)
	})

	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%d, want 3 (2 retries + success)", len(events))
	}
	if events[0].Status != 503 || events[1].Status != 503 || events[2].Status != 200 {
		t.Fatalf("status sequence: %d %d %d", events[0].Status, events[1].Status, events[2].Status)
	}
	if events[0].Retry != 0 || events[1].Retry != 1 || events[2].Retry != 2 {
		t.Fatalf("retry sequence: %d %d %d", events[0].Retry, events[1].Retry, events[2].Retry)
	}
}

func TestAuditorPanicSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, auth.NewAPIKey("k"))
	c.Audit = AuditorFunc(func(context.Context, AuditEvent) { panic("boom") })

	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
}
