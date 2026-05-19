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

func TestTracerEmitsSpan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var spans []Span
	var mu sync.Mutex
	c := New(srv.URL, auth.NewAPIKey("k"))
	c.Tracer = TracerFunc(func(ctx context.Context, s Span) {
		mu.Lock()
		spans = append(spans, s)
		mu.Unlock()
	})

	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("spans=%d, want 1", len(spans))
	}
	s := spans[0]
	if s.StatusCode != "OK" {
		t.Fatalf("status=%s", s.StatusCode)
	}
	if s.Attributes["http.method"] != "GET" {
		t.Fatalf("method attr=%v", s.Attributes["http.method"])
	}
	if s.TraceID == "" || s.SpanID == "" {
		t.Fatalf("ids missing: %+v", s)
	}
}

func TestTracerEmitsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "x", http.StatusNotFound)
	}))
	defer srv.Close()

	var got Span
	c := New(srv.URL, auth.NewAPIKey("k"))
	c.MaxRetries = 0
	c.Tracer = TracerFunc(func(ctx context.Context, s Span) { got = s })

	_ = c.Get(context.Background(), "/x", nil, nil)
	if got.StatusCode != "ERROR" {
		t.Fatalf("status=%s", got.StatusCode)
	}
	if got.Attributes["http.status_code"] != 404 {
		t.Fatalf("status_code attr=%v", got.Attributes["http.status_code"])
	}
}

func TestTracerSharesTraceIDAcrossRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var spans []Span
	c := New(srv.URL, auth.NewAPIKey("k"))
	c.MaxRetries = 5
	c.Tracer = TracerFunc(func(ctx context.Context, s Span) { spans = append(spans, s) })

	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(spans) < 2 {
		t.Fatalf("spans=%d, want >=2", len(spans))
	}
	for i, s := range spans[1:] {
		if s.TraceID != spans[0].TraceID {
			t.Errorf("span %d trace_id differs: %s vs %s", i+1, s.TraceID, spans[0].TraceID)
		}
		if s.SpanID == spans[0].SpanID {
			t.Errorf("span %d shares span_id with span 0", i+1)
		}
	}
}
