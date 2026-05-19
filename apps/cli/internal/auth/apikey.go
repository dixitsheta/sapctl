package auth

import (
	"context"
	"net/http"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

// APIKey injects the SAP Business Accelerator Hub sandbox APIKey header.
//
// The SAP sandbox uses a non-standard header name (`APIKey`, not `X-API-Key`
// or `Authorization`). This implementation enforces that header verbatim.
type APIKey struct {
	key string
}

// NewAPIKey constructs the provider. An empty key is allowed at construction
// time but will be rejected on Apply so that errors surface inside the
// command rather than during config loading.
func NewAPIKey(key string) *APIKey {
	return &APIKey{key: key}
}

func (a *APIKey) Name() string { return "apikey" }

func (a *APIKey) Apply(ctx context.Context, req *http.Request) error {
	if a.key == "" {
		return errs.New(errs.ExitAuth, "auth.apikey.empty", "API key is empty; run `sapctl auth login --flow apikey`")
	}
	req.Header.Set("APIKey", a.key)
	return nil
}
