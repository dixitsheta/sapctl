// Package auth provides pluggable authentication providers for SAP services.
//
// Providers implement the Provider interface and inject credentials into
// outgoing http.Request objects. Tokens are cached on disk via the cache
// package so that interactive logins are not required on every command.
//
// Locked providers (ADR 0002):
//   - apikey   : SAP Business Accelerator Hub sandbox APIKey header
//   - basic    : HTTP Basic auth (S/4HANA Cloud Communication User)
//   - xsuaa    : OAuth2 client_credentials against BTP XSUAA / IAS
//
// Future providers (ADR 0001 timeline):
//   - samlbearer
//   - cert
//   - hana-sql
package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Provider authenticates outgoing HTTP requests for a given SAP service.
//
// Implementations must be safe for concurrent use. Apply may perform a
// blocking token-refresh; callers should pass a context with a timeout when
// invoking it from latency-sensitive paths.
type Provider interface {
	// Name returns the stable provider identifier (e.g. "apikey", "xsuaa").
	Name() string

	// Apply mutates req to carry the credential. It MAY perform a network
	// call (e.g. OAuth token refresh). Errors returned from Apply should be
	// wrapped errs.Error values with ExitAuth as their code.
	Apply(ctx context.Context, req *http.Request) error
}

// Config is the on-disk credential descriptor for a single provider.
// Subset of fields are used per-provider; unused fields stay empty.
type Config struct {
	Provider string `json:"provider" yaml:"provider"`

	// apikey
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`

	// basic
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	// xsuaa
	ClientID     string `json:"client_id,omitempty"     yaml:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	TokenURL     string `json:"token_url,omitempty"     yaml:"token_url,omitempty"`

	// shared (label for audit + UX)
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
}

// New builds a Provider from a Config. Unknown provider names return an error.
func New(c Config) (Provider, error) {
	switch c.Provider {
	case "apikey":
		return NewAPIKey(c.APIKey), nil
	case "basic":
		return NewBasic(c.Username, c.Password), nil
	case "xsuaa":
		return NewXSUAA(c.ClientID, c.ClientSecret, c.TokenURL), nil
	default:
		return nil, fmt.Errorf("auth: unknown provider %q", c.Provider)
	}
}
