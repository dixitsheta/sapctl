package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
)

// btpFakeOAuth runs a fake XSUAA token endpoint that hands out a static JWT.
func btpFakeOAuth(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"jwt-xyz","token_type":"bearer","expires_in":3600}`))
	}))
}

// btpSeedXSUAACred stages a cached xsuaa credential with the given token URL.
func btpSeedXSUAACred(t *testing.T, label, tokenURL string) {
	t.Helper()
	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	cfg := auth.Config{
		Provider:     "xsuaa",
		ClientID:     "cid",
		ClientSecret: "csecret",
		TokenURL:     tokenURL,
	}
	if err := cache.Save(label, cfg); err != nil {
		t.Fatalf("seed creds: %v", err)
	}
}

func TestBTPSubaccountList(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-xyz" {
			t.Errorf("Authorization=%q", got)
		}
		if r.URL.Path != "/accounts/v1/subaccounts" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$top"); got != "2" {
			t.Errorf("$top=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[
			{"guid":"g1","displayName":"Trial A","subdomain":"trial-a","region":"eu10","state":"OK"},
			{"guid":"g2","displayName":"Trial B","subdomain":"trial-b","region":"eu10","state":"OK"}
		]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "subaccount", "list",
		"--cred", "btp-trial",
		"--api", api.URL,
		"--top", "2",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var got struct {
		Count       int `json:"count"`
		Subaccounts []struct {
			GUID, DisplayName string
		} `json:"subaccounts"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 2 || got.Subaccounts[0].GUID != "g1" {
		t.Fatalf("unexpected output: %+v", got)
	}
}

func TestBTPSubaccountGet(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/accounts/v1/subaccounts/abc-123") {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"guid":"abc-123","displayName":"My Trial","subdomain":"my-trial","region":"eu10","state":"OK"}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "subaccount", "get", "abc-123",
		"--cred", "btp-trial",
		"--api", api.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var got struct {
		Count       int `json:"count"`
		Subaccounts []struct {
			GUID, DisplayName string
		} `json:"subaccounts"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 1 || got.Subaccounts[0].GUID != "abc-123" {
		t.Fatalf("output: %+v", got)
	}
}

func TestBTPMissingCred(t *testing.T) {
	withTempCache(t)
	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"btp", "subaccount", "list"})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error when --cred missing")
	}
}

func TestBTPWrongFlow(t *testing.T) {
	withTempCache(t)
	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"btp", "subaccount", "list", "--cred", "sandbox"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "expected flow=xsuaa") {
		t.Fatalf("expected wrong-flow error, got %v", err)
	}
}

func TestBTPServiceInstanceList(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/service_instances" {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"si-1","name":"my-xsuaa","service_id":"sid","service_plan_id":"plan","ready":true,"usable":true}
		]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "service-instance", "list",
		"--cred", "btp-trial",
		"--api", api.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var got struct {
		Count            int `json:"count"`
		ServiceInstances []struct {
			ID, Name string
		} `json:"service_instances"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 1 || got.ServiceInstances[0].ID != "si-1" {
		t.Fatalf("output: %+v", got)
	}
}
