package auth

import (
	"context"
	"net/http"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

// Basic implements HTTP Basic authentication for S/4HANA Cloud Communication
// Users and similar password-based SAP endpoints.
type Basic struct {
	user, pass string
}

func NewBasic(user, pass string) *Basic {
	return &Basic{user: user, pass: pass}
}

func (b *Basic) Name() string { return "basic" }

func (b *Basic) Apply(ctx context.Context, req *http.Request) error {
	if b.user == "" || b.pass == "" {
		return errs.New(errs.ExitAuth, "auth.basic.empty",
			"basic auth credentials are empty; run `sapctl auth login --flow basic`")
	}
	req.SetBasicAuth(b.user, b.pass)
	return nil
}
