package sap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"time"
)

// Tracer receives one Span per HTTP attempt. Implementations MUST be
// non-blocking and panic-safe; sap.Client treats Tracer panics as advisory.
type Tracer interface {
	Emit(ctx context.Context, span Span)
}

// Span is an OTLP-shaped representation of one HTTP attempt. Field names
// match OTel HTTP semantic conventions where practical, with a flat shape
// for JSON-line output.
type Span struct {
	Name       string           `json:"name"`
	TraceID    string           `json:"trace_id"`
	SpanID     string           `json:"span_id"`
	ParentID   string           `json:"parent_span_id,omitempty"`
	StartTime  time.Time        `json:"start_time"`
	EndTime    time.Time        `json:"end_time"`
	StatusCode string           `json:"status_code"` // "OK" or "ERROR"
	Attributes map[string]any   `json:"attributes,omitempty"`
	Events     []map[string]any `json:"events,omitempty"`
}

// TracerFunc adapts a plain func to Tracer.
type TracerFunc func(ctx context.Context, span Span)

func (f TracerFunc) Emit(ctx context.Context, span Span) { f(ctx, span) }

// NewJSONTracer writes one JSON line per span to w.
func NewJSONTracer(w io.Writer) Tracer {
	enc := json.NewEncoder(w)
	return TracerFunc(func(ctx context.Context, s Span) {
		_ = enc.Encode(s)
	})
}

// NewEnvTracer returns a stderr JSON tracer when SAPCTL_TRACE=1, else nil.
// Callers use it like:
//
//	if t := sap.NewEnvTracer(); t != nil { client.Tracer = t }
func NewEnvTracer() Tracer {
	if os.Getenv("SAPCTL_TRACE") != "1" {
		return nil
	}
	return NewJSONTracer(os.Stderr)
}

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func newSpanID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
