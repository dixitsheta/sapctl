// Package sap provides a generic, auth-aware HTTP client for SAP REST/OData
// endpoints. The client handles:
//
//   - injection of credentials via an auth.Provider
//   - gzip decompression (handled by net/http when Accept-Encoding is set)
//   - bounded retries on 429/5xx with exponential backoff + jitter
//   - JSON body decoding helpers
//   - mapping HTTP status to locked sapctl exit codes
//
// All SAP product subcommands (s4, btp, datasphere, ...) MUST use this client
// rather than the raw net/http package.
package sap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

// Client wraps http.Client and injects sapctl conventions.
type Client struct {
	BaseURL    string
	Auth       auth.Provider
	HTTP       *http.Client
	UserAgent  string
	MaxRetries int
	// Audit, if non-nil, receives one AuditEvent per HTTP request cycle
	// (including retries -- each attempt is its own event with Retry set).
	Audit Auditor
	// Tracer, if non-nil, receives one Span per HTTP attempt.
	Tracer Tracer
	// TraceID, if set, is used as the parent trace id for all spans emitted
	// by this client. Leave empty to generate a fresh trace per request.
	TraceID string
}

// New builds a client with sensible defaults.
func New(baseURL string, p auth.Provider) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Auth:       p,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
		UserAgent:  "sapctl/0.0.0-dev",
		MaxRetries: 3,
	}
}

// Get issues a GET against path (joined with BaseURL) and decodes JSON into out.
// Pass nil for out to discard the body.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.do(ctx, http.MethodGet, u, nil, out)
}

func (c *Client) do(ctx context.Context, method, u string, body io.Reader, out any) error {
	var lastErr error
	traceID := c.TraceID
	if traceID == "" {
		traceID = newTraceID()
	}
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errs.Wrap(errs.ExitUserError, "sap.ctx", "context cancelled", ctx.Err())
			case <-time.After(backoff(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, u, body)
		if err != nil {
			return errs.Wrap(errs.ExitUserError, "sap.request", "build request", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)

		if c.Auth != nil {
			if err := c.Auth.Apply(ctx, req); err != nil {
				return err
			}
		}

		started := time.Now()
		spanID := newSpanID()
		resp, err := c.HTTP.Do(req)
		dur := time.Since(started)
		end := started.Add(dur)
		c.recordTrace(ctx, Span{
			Name:       method + " " + req.URL.Path,
			TraceID:    traceID,
			SpanID:     spanID,
			StartTime:  started,
			EndTime:    end,
			StatusCode: spanStatus(resp, err),
			Attributes: map[string]any{
				"http.method":      method,
				"http.url":         u,
				"http.status_code": statusOf(resp),
				"http.retry":       attempt,
				"sapctl.provider":  c.providerName(),
			},
		})

		if err != nil {
			lastErr = err
			c.recordAudit(ctx, AuditEvent{
				Method: method, URL: u, Duration: dur, UserAgent: c.UserAgent,
				Retry: attempt, Error: err.Error(), Provider: c.providerName(),
			})
			if isRetryableNetErr(err) {
				continue
			}
			return errs.Wrap(errs.ExitUserError, "sap.network", "transport error", err)
		}

		if shouldRetryStatus(resp.StatusCode) && attempt < c.MaxRetries {
			n, _ := io.Copy(io.Discard, resp.Body)
			c.recordAudit(ctx, AuditEvent{
				Method: method, URL: u, Status: resp.StatusCode, Duration: dur,
				RequestID: requestID(resp), RespBytes: int(n),
				UserAgent: c.UserAgent, Retry: attempt, Provider: c.providerName(),
			})
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable status %d", resp.StatusCode)
			continue
		}

		ev := AuditEvent{
			Method: method, URL: u, Status: resp.StatusCode, Duration: dur,
			RequestID: requestID(resp), UserAgent: c.UserAgent, Retry: attempt,
			Provider: c.providerName(),
		}
		err = decodeResponse(resp, out)
		if err != nil {
			ev.Error = err.Error()
		}
		c.recordAudit(ctx, ev)
		return err
	}
	if lastErr != nil {
		return errs.Wrap(errs.ExitUserError, "sap.retry_exhausted", "retries exhausted", lastErr)
	}
	return errs.New(errs.ExitUserError, "sap.unknown", "unknown error in HTTP loop")
}

// recordAudit safely dispatches an event to the configured Auditor. Panics
// inside the Auditor are recovered so audit failures never break the request.
func (c *Client) recordAudit(ctx context.Context, ev AuditEvent) {
	if c.Audit == nil {
		return
	}
	defer func() { _ = recover() }()
	c.Audit.Record(ctx, ev)
}

func (c *Client) providerName() string {
	if c.Auth == nil {
		return ""
	}
	return c.Auth.Name()
}

// recordTrace safely dispatches a span to the configured Tracer. Panics in
// the tracer are recovered so trace failures never break the request.
func (c *Client) recordTrace(ctx context.Context, s Span) {
	if c.Tracer == nil {
		return
	}
	defer func() { _ = recover() }()
	c.Tracer.Emit(ctx, s)
}

// spanStatus maps an HTTP response / err pair to OTel-style "OK" or "ERROR".
func spanStatus(resp *http.Response, err error) string {
	if err != nil {
		return "ERROR"
	}
	if resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "ERROR"
	}
	return "OK"
}

// statusOf returns the numeric HTTP status or 0 if resp is nil.
func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func decodeResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "sap.read", "read response body", err)
	}

	urlPath := ""
	if resp.Request != nil && resp.Request.URL != nil {
		urlPath = resp.Request.URL.Path
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errs.New(errs.ExitAuth, "sap.unauthorized",
			fmt.Sprintf("HTTP 401 from %s: %s", urlPath, truncate(string(body), 200)))
	case resp.StatusCode == http.StatusNotFound:
		return errs.New(errs.ExitNotFound, "sap.not_found",
			fmt.Sprintf("HTTP 404 from %s", urlPath))
	case resp.StatusCode == http.StatusConflict:
		return errs.New(errs.ExitConflict, "sap.conflict",
			fmt.Sprintf("HTTP 409 from %s: %s", urlPath, truncate(string(body), 200)))
	case resp.StatusCode == http.StatusTooManyRequests:
		return errs.New(errs.ExitRateLimited, "sap.rate_limited",
			fmt.Sprintf("HTTP 429 from %s", urlPath))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return errs.New(errs.ExitUserError, "sap.http",
			fmt.Sprintf("HTTP %d from %s: %s", resp.StatusCode, urlPath, truncate(string(body), 200)))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errs.Wrap(errs.ExitUserError, "sap.decode",
			fmt.Sprintf("decode JSON from %s", urlPath), err)
	}
	return nil
}

func shouldRetryStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func isRetryableNetErr(err error) bool { return err != nil }

func backoff(attempt int) time.Duration {
	base := time.Duration(1<<attempt) * 200 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base / 2))) //nolint:gosec
	return base + jitter
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
