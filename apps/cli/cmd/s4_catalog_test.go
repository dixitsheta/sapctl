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

// withTempCache redirects XDG_CONFIG_HOME so auth.DefaultPath() lands in t.TempDir.
func withTempCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestS4CatalogDiscoverSandbox(t *testing.T) {
	withTempCache(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("APIKey") != "fake-key" {
			t.Errorf("missing APIKey")
		}
		if !strings.Contains(r.URL.Path, "CATALOGSERVICE") {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$top"); got != "3" {
			t.Errorf("$top=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[
			{"ID":"S1","Title":"Svc1","Author":"sap"},
			{"ID":"S2","Title":"Svc2","Author":"sap"}
		]}}`))
	}))
	defer srv.Close()

	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	if err := cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "fake-key"}); err != nil {
		t.Fatalf("seed creds: %v", err)
	}

	globalFlags = GlobalFlags{}
	s4Shared = s4Flags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"s4", "catalog", "discover",
		"--cred", "sandbox",
		"--base-url", srv.URL,
		"--top", "3",
		"--json",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var got struct {
		Count    int `json:"count"`
		Services []struct {
			ID, Title string
		} `json:"services"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode out: %v\n%s", err, out.String())
	}
	if got.Count != 2 || got.Services[0].ID != "S1" {
		t.Fatalf("unexpected output: %+v", got)
	}
}

func TestS4CatalogMissingCred(t *testing.T) {
	withTempCache(t)
	globalFlags = GlobalFlags{}
	s4Shared = s4Flags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"s4", "catalog", "discover"})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error when --cred missing")
	}
}
