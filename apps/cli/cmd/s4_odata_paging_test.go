package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
)

// TestS4ODataPaginationAll verifies --all follows d.__next across pages.
func TestS4ODataPaginationAll(t *testing.T) {
	withTempCache(t)

	var hits int32
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			_, _ = w.Write([]byte(`{"d":{"results":[
				{"BP":"1"},{"BP":"2"}
			],"__next":"` + srvURL + `/page2"}}`))
		case 2:
			if !strings.HasSuffix(r.URL.Path, "/page2") {
				t.Errorf("page2 path=%q", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"d":{"results":[
				{"BP":"3"},{"BP":"4"}
			],"__next":"` + srvURL + `/page3"}}`))
		case 3:
			_, _ = w.Write([]byte(`{"d":{"results":[
				{"BP":"5"}
			]}}`))
		default:
			t.Errorf("unexpected hit %d", n)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

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
		"--service", "API_BP",
		"--entity", "A_BP",
		"--all",
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
	if got.Count != 5 {
		t.Fatalf("count=%d, want 5 (3 pages aggregated)", got.Count)
	}
	if atomic.LoadInt32(&hits) != 3 {
		t.Fatalf("hits=%d, want 3", hits)
	}
}

// TestS4ODataPaginationDefault confirms that without --all only first page is fetched.
func TestS4ODataPaginationDefault(t *testing.T) {
	withTempCache(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[{"BP":"1"}],"__next":"http://example.invalid/should-not-follow"}}`))
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
		"--service", "API_BP",
		"--entity", "A_BP",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("hits=%d, want 1 (default should NOT follow next)", hits)
	}
}

// TestNextPathCrossHostRejected verifies the SSRF guard.
func TestNextPathCrossHostRejected(t *testing.T) {
	_, err := nextPath("https://sandbox.api.sap.com", "https://attacker.example/loot")
	if err == nil {
		t.Fatalf("expected cross-host rejection")
	}
}

// TestNextPathSameHost extracts the path correctly.
func TestNextPathSameHost(t *testing.T) {
	p, err := nextPath("https://x.example", "https://x.example/path?$skiptoken=abc")
	if err != nil {
		t.Fatalf("nextPath: %v", err)
	}
	if p != "/path?$skiptoken=abc" {
		t.Fatalf("path=%q", p)
	}
}
