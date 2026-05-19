package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBTPServiceBindingCreate(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/service_bindings" {
			t.Errorf("path=%q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		if got["name"] != "my-binding" || got["service_instance_id"] != "si-1" {
			t.Errorf("body=%+v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "b-1", "name": "my-binding", "service_instance_id": "si-1",
			"ready": true,
			"credentials": {"clientid":"abc","clientsecret":"xyz","url":"https://x.eu10.hana.ondemand.com"}
		}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "service-binding", "create",
		"--cred", "btp-trial",
		"--api", api.URL,
		"--name", "my-binding",
		"--instance-id", "si-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var resp struct {
		Count           int `json:"count"`
		ServiceBindings []struct {
			ID, Name    string
			Credentials map[string]string
		} `json:"service_bindings"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if resp.Count != 1 || resp.ServiceBindings[0].ID != "b-1" {
		t.Fatalf("output: %+v", resp)
	}
	if resp.ServiceBindings[0].Credentials["clientid"] != "abc" {
		t.Fatalf("credentials missing: %+v", resp.ServiceBindings[0])
	}
}

func TestBTPServiceBindingDelete(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("want DELETE, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v2/service_bindings/b-1") {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "service-binding", "delete", "b-1",
		"--cred", "btp-trial",
		"--api", api.URL,
		"--yes",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), "deleted binding b-1") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestBTPServiceBindingDeleteRequiresYes(t *testing.T) {
	withTempCache(t)
	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "btp-trial", oauth.URL+"/oauth/token")

	globalFlags = GlobalFlags{}
	btpShared = btpFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"btp", "service-binding", "delete", "b-1",
		"--cred", "btp-trial",
		"--api", "http://localhost",
	})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error without --yes")
	}
}
