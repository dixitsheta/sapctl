package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
)

func TestDatasphereSpaceList(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ds-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/dwc/consumption/spaces" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer jwt-xyz" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[
			{"id":"s1","name":"sales","display_name":"Sales mart","state":"OK"},
			{"id":"s2","name":"hr","display_name":"HR mart","state":"OK"}
		]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	dsShared = dsFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"datasphere", "space", "list",
		"--cred", "ds-trial",
		"--api", api.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	var got struct {
		Count  int `json:"count"`
		Spaces []struct {
			ID, Name string
		} `json:"spaces"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 2 || got.Spaces[0].ID != "s1" {
		t.Fatalf("output: %+v", got)
	}
}

func TestDatasphereSQLExec(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ds-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/v1/dwc/consumption/spaces/sales/sql") {
			t.Errorf("path=%q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var p map[string]string
		_ = json.Unmarshal(body, &p)
		if !strings.Contains(p["query"], "SELECT 1") {
			t.Errorf("query body=%v", p)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["c"],"rows":[[1]]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	dsShared = dsFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"datasphere", "sql", "exec",
		"--cred", "ds-trial",
		"--api", api.URL,
		"--space", "sales",
		"--query", "SELECT 1 FROM DUMMY",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), `"columns"`) {
		t.Fatalf("missing pass-through body: %s", out.String())
	}
}

func TestDatasphereMissingApi(t *testing.T) {
	withTempCache(t)
	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ds-trial", oauth.URL+"/oauth/token")

	globalFlags = GlobalFlags{}
	dsShared = dsFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"datasphere", "space", "list",
		"--cred", "ds-trial",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api is required") {
		t.Fatalf("expected --api required error, got %v", err)
	}
}

func TestDatasphereWrongFlow(t *testing.T) {
	withTempCache(t)
	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	globalFlags = GlobalFlags{}
	dsShared = dsFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"datasphere", "space", "list",
		"--cred", "sandbox",
		"--api", "http://localhost",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "expected flow=xsuaa") {
		t.Fatalf("expected wrong-flow error, got %v", err)
	}
}
