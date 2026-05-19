package sap

import (
	"context"
	"net/http"
	"time"
)

// Auditor receives one Event per HTTP request/response cycle. Implementations
// MUST be non-blocking and MUST NOT panic; sap.Client treats Auditor errors
// as advisory and continues regardless.
type Auditor interface {
	Record(ctx context.Context, ev AuditEvent)
}

// AuditEvent is the payload emitted for one HTTP transaction.
//
// PII guidance: the URL path + query are recorded but the bearer token /
// APIKey header MUST NOT be present (sap.Client never logs auth headers).
type AuditEvent struct {
	Method    string        `json:"method"`
	URL       string        `json:"url"`
	Status    int           `json:"status"`
	Duration  time.Duration `json:"duration_ns"`
	RequestID string        `json:"request_id,omitempty"`
	RespBytes int           `json:"resp_bytes"`
	UserAgent string        `json:"user_agent"`
	Retry     int           `json:"retry"`
	Error     string        `json:"error,omitempty"`
	Provider  string        `json:"provider,omitempty"`
}

// AuditorFunc adapts a plain func to the Auditor interface.
type AuditorFunc func(ctx context.Context, ev AuditEvent)

func (f AuditorFunc) Record(ctx context.Context, ev AuditEvent) { f(ctx, ev) }

// requestID extracts a server-assigned correlation id from response headers.
func requestID(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	for _, h := range []string{"X-Request-Id", "X-Correlationid", "Sap-Passport"} {
		if v := resp.Header.Get(h); v != "" {
			return v
		}
	}
	return ""
}
