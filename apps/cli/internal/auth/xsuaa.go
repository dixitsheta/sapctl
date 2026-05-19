package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

// XSUAA implements OAuth2 client_credentials against BTP XSUAA / IAS token
// endpoints. Acquired tokens are cached in memory for their lifetime minus a
// small safety window.
type XSUAA struct {
	clientID     string
	clientSecret string
	tokenURL     string

	httpClient *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

type xsuaaTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// safetyWindow is subtracted from the server-provided expiry so that callers
// never present a token the server is about to reject.
const safetyWindow = 30 * time.Second

// NewXSUAA constructs the provider. tokenURL must include the /oauth/token
// path; the XSUAA binding `url` field plus this suffix is the canonical form.
func NewXSUAA(clientID, clientSecret, tokenURL string) *XSUAA {
	return &XSUAA{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// SetHTTPClient overrides the default client. Used by tests.
func (x *XSUAA) SetHTTPClient(c *http.Client) { x.httpClient = c }

func (x *XSUAA) Name() string { return "xsuaa" }

func (x *XSUAA) Apply(ctx context.Context, req *http.Request) error {
	tok, err := x.fetchToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

func (x *XSUAA) fetchToken(ctx context.Context) (string, error) {
	x.mu.Lock()
	defer x.mu.Unlock()

	if x.token != "" && time.Now().Before(x.expiry) {
		return x.token, nil
	}

	if x.clientID == "" || x.clientSecret == "" || x.tokenURL == "" {
		return "", errs.New(errs.ExitAuth, "auth.xsuaa.empty",
			"XSUAA credentials missing; run `sapctl auth login --flow xsuaa`")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, x.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errs.Wrap(errs.ExitAuth, "auth.xsuaa.request", "build token request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(x.clientID, x.clientSecret)

	resp, err := x.httpClient.Do(req)
	if err != nil {
		return "", errs.Wrap(errs.ExitAuth, "auth.xsuaa.network", "token request failed", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errs.New(errs.ExitAuth, "auth.xsuaa.http",
			fmt.Sprintf("token endpoint returned %d: %s", resp.StatusCode, truncate(string(body), 200)))
	}

	var tr xsuaaTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", errs.Wrap(errs.ExitAuth, "auth.xsuaa.decode", "decode token response", err)
	}
	if tr.AccessToken == "" {
		return "", errs.New(errs.ExitAuth, "auth.xsuaa.empty_token", "token endpoint returned empty access_token")
	}

	x.token = tr.AccessToken
	x.expiry = time.Now().Add(time.Duration(tr.ExpiresIn)*time.Second - safetyWindow)
	return x.token, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
