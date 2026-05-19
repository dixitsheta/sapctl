package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	sqlitemirror "github.com/dixitsheta/sapctl/packages/sqlite-mirror"
)

// TestS4ODataSinceFirstRun verifies first call with --since-field + --mirror
// sends no $filter (no prior watermark) and sets watermark = max(rows).
func TestS4ODataSinceFirstRun(t *testing.T) {
	withTempCache(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$filter") != "" {
			t.Errorf("first run should not send $filter; got %q", r.URL.Query().Get("$filter"))
		}
		if r.URL.Query().Get("$orderby") != "LastChangeDateTime asc" {
			t.Errorf("expected default orderby on since-field, got %q", r.URL.Query().Get("$orderby"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[
			{"BP":"1","LastChangeDateTime":"2026-05-18T10:00:00Z"},
			{"BP":"2","LastChangeDateTime":"2026-05-19T09:30:00Z"}
		]}}`))
	}))
	defer srv.Close()

	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	dbPath := filepath.Join(t.TempDir(), "mirror.db")

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
		"--top", "10",
		"--mirror", dbPath,
		"--key-field", "BP",
		"--since-field", "LastChangeDateTime",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	store, err := sqlitemirror.Open(dbPath)
	if err != nil {
		t.Fatalf("Open mirror: %v", err)
	}
	defer store.Close()

	got, err := store.GetWatermark(context.Background(), "API_BUSINESS_PARTNER", "A_BusinessPartner")
	if err != nil {
		t.Fatalf("GetWatermark: %v", err)
	}
	if got != "2026-05-19T09:30:00Z" {
		t.Fatalf("watermark=%q, want 2026-05-19T09:30:00Z", got)
	}

	n, _ := store.Count(context.Background(), "API_BUSINESS_PARTNER", "A_BusinessPartner")
	if n != 2 {
		t.Fatalf("rows=%d, want 2", n)
	}
}

// TestS4ODataSinceSecondRun verifies a second call sends $filter from stored
// watermark and advances the cursor.
func TestS4ODataSinceSecondRun(t *testing.T) {
	withTempCache(t)

	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	store, _ := sqlitemirror.Open(dbPath)
	_ = store.SetWatermark(context.Background(),
		"API_BUSINESS_PARTNER", "A_BusinessPartner", "2026-05-19T09:30:00Z")
	_ = store.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("$filter")
		want := "LastChangeDateTime gt datetimeoffset'2026-05-19T09:30:00Z'"
		if got != want {
			t.Errorf("$filter=%q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[
			{"BP":"3","LastChangeDateTime":"2026-05-19T12:00:00Z"}
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
		"--mirror", dbPath,
		"--key-field", "BP",
		"--since-field", "LastChangeDateTime",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	store, _ = sqlitemirror.Open(dbPath)
	defer store.Close()
	got, _ := store.GetWatermark(context.Background(), "API_BUSINESS_PARTNER", "A_BusinessPartner")
	if got != "2026-05-19T12:00:00Z" {
		t.Fatalf("watermark=%q, want 2026-05-19T12:00:00Z", got)
	}
}

// TestS4ODataSinceReset verifies --since-reset ignores the stored watermark.
func TestS4ODataSinceReset(t *testing.T) {
	withTempCache(t)

	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	store, _ := sqlitemirror.Open(dbPath)
	_ = store.SetWatermark(context.Background(),
		"API_BP", "A_BP", "2026-05-19T09:30:00Z")
	_ = store.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$filter") != "" {
			t.Errorf("--since-reset should NOT send $filter; got %q", r.URL.Query().Get("$filter"))
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
		"--service", "API_BP",
		"--entity", "A_BP",
		"--mirror", dbPath,
		"--since-field", "LastChangeDateTime",
		"--since-reset",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
}

// TestMirrorReset verifies the `sapctl mirror reset --yes` flow.
func TestMirrorReset(t *testing.T) {
	withTempCache(t)
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	store, _ := sqlitemirror.Open(dbPath)
	ctx := context.Background()
	_ = store.Upsert(ctx, "S", "E", "k", []byte(`{"v":1}`))
	_ = store.SetWatermark(ctx, "S", "E", "2026-05-19T00:00:00Z")
	_ = store.Close()

	globalFlags = GlobalFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"mirror", "reset",
		"--db", dbPath,
		"--service", "S",
		"--entity", "E",
		"--yes",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), `"deleted": 1`) {
		t.Fatalf("expected deleted=1 in output: %s", out.String())
	}
}

func TestMirrorResetRequiresYes(t *testing.T) {
	withTempCache(t)
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	store, _ := sqlitemirror.Open(dbPath)
	_ = store.Close()

	globalFlags = GlobalFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"mirror", "reset",
		"--db", dbPath,
		"--service", "S",
		"--entity", "E",
	})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error without --yes")
	}
}
