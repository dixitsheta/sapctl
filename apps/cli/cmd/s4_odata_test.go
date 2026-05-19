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

func TestS4ODataGetCollection(t *testing.T) {
	withTempCache(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("APIKey") != "k" {
			t.Errorf("missing APIKey")
		}
		if !strings.HasSuffix(r.URL.Path, "/A_BusinessPartner") {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$top"); got != "2" {
			t.Errorf("$top=%q", got)
		}
		if got := r.URL.Query().Get("$select"); got != "BusinessPartner,FullName" {
			t.Errorf("$select=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[
			{"BusinessPartner":"1","FullName":"Alice"},
			{"BusinessPartner":"2","FullName":"Bob"}
		]}}`))
	}))
	defer srv.Close()

	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	globalFlags = GlobalFlags{}
	s4Shared = s4Flags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"s4", "odata", "get",
		"--cred", "sandbox",
		"--base-url", srv.URL,
		"--service", "API_BUSINESS_PARTNER",
		"--entity", "A_BusinessPartner",
		"--top", "2",
		"--select-fields", "BusinessPartner,FullName",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var got struct {
		Count int               `json:"count"`
		Rows  []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 2 {
		t.Fatalf("count=%d, want 2", got.Count)
	}
	if !strings.Contains(string(got.Rows[0]), `"Alice"`) {
		t.Fatalf("row0=%s", got.Rows[0])
	}
	var row0 map[string]string
	if err := json.Unmarshal(got.Rows[0], &row0); err != nil {
		t.Fatalf("row0 decode: %v", err)
	}
	if row0["FullName"] != "Alice" {
		t.Fatalf("FullName=%q", row0["FullName"])
	}
}

func TestS4ODataGetServicePathPassthrough(t *testing.T) {
	withTempCache(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Full path should NOT be re-prefixed.
		if !strings.HasPrefix(r.URL.Path, "/custom/path/SVC/Entity") {
			t.Errorf("unexpected path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[]}}`))
	}))
	defer srv.Close()

	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	globalFlags = GlobalFlags{}
	s4Shared = s4Flags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"s4", "odata", "get",
		"--cred", "sandbox",
		"--base-url", srv.URL,
		"--service", "/custom/path/SVC",
		"--entity", "Entity",
		"--top", "1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
}

func TestExtractRowsSingleton(t *testing.T) {
	rows, err := extractRows(json.RawMessage(`{"id":"X"}`))
	if err != nil {
		t.Fatalf("extractRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
}
