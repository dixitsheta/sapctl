package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixitsheta/sapctl/apps/cli/internal/auth"
	auditchain "github.com/dixitsheta/sapctl/packages/audit-chain"
)

func TestS4AuditExportSOXJournal(t *testing.T) {
	withTempCache(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "API_JOURNALENTRYITEMBASIC_SRV/A_JournalEntryItemBasic") {
			t.Errorf("path=%q", r.URL.Path)
		}
		got := r.URL.Query().Get("$filter")
		if !strings.Contains(got, "PostingDate ge datetimeoffset'2026-01-01T00:00:00Z'") {
			t.Errorf("filter missing lower bound: %q", got)
		}
		if !strings.Contains(got, "PostingDate le datetimeoffset'2026-03-31T23:59:59Z'") {
			t.Errorf("filter missing upper bound: %q", got)
		}
		if r.URL.Query().Get("$orderby") != "PostingDate asc" {
			t.Errorf("orderby=%q", r.URL.Query().Get("$orderby"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"d":{"results":[
			{"JournalEntry":"1","PostingDate":"/Date(1672531200000)/","GLAccount":"40000000"},
			{"JournalEntry":"2","PostingDate":"/Date(1672617600000)/","GLAccount":"40000001"},
			{"JournalEntry":"3","PostingDate":"/Date(1672704000000)/","GLAccount":"40000002"}
		]}}`))
	}))
	defer srv.Close()

	path, _ := auth.DefaultPath()
	cache := auth.NewCache(path)
	_ = cache.Save("sandbox", auth.Config{Provider: "apikey", APIKey: "k"})

	outDir := t.TempDir()

	globalFlags = GlobalFlags{}
	s4Shared = s4Flags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"s4", "audit-export",
		"--cred", "sandbox",
		"--base-url", srv.URL,
		"--use-case", "sox-journal",
		"--from", "2026-01-01",
		"--to", "2026-03-31",
		"--out", outDir,
		"--max-rows", "10",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var result struct {
		UseCase  string `json:"use_case"`
		RowCount int    `json:"row_count"`
		WorkDir  string `json:"work_dir"`
		Bundle   string `json:"bundle"`
		Manifest string `json:"manifest"`
		Chain    string `json:"chain"`
		PubKey   string `json:"public_key"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode out: %v\n%s", err, out.String())
	}
	if result.UseCase != "sox-journal" {
		t.Fatalf("use_case=%q", result.UseCase)
	}
	if result.RowCount != 3 {
		t.Fatalf("row_count=%d, want 3", result.RowCount)
	}

	rowsBytes, err := os.ReadFile(filepath.Join(result.WorkDir, "rows.jsonl"))
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	lines := strings.Count(strings.TrimRight(string(rowsBytes), "\n"), "\n") + 1
	if lines != 3 {
		t.Fatalf("rows.jsonl lines=%d, want 3", lines)
	}

	mbytes, err := os.ReadFile(result.Manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(mbytes, &manifest); err != nil {
		t.Fatalf("manifest decode: %v", err)
	}
	for _, k := range []string{"use_case", "from", "to", "row_count", "sha256_rows", "sha256_chain", "generated_at"} {
		if _, ok := manifest[k]; !ok {
			t.Errorf("manifest missing %q", k)
		}
	}
	if int(manifest["row_count"].(float64)) != 3 {
		t.Fatalf("manifest row_count=%v", manifest["row_count"])
	}

	pub, err := auditchain.LoadPublicKey(result.PubKey)
	if err != nil {
		t.Fatalf("load pub: %v", err)
	}
	n, err := auditchain.Verify(result.Chain, pub)
	if err != nil {
		t.Fatalf("chain verify: %v", err)
	}
	// 3 rows + start + end envelopes = 5 events
	if n != 5 {
		t.Fatalf("verified events=%d, want 5 (3 rows + start + end)", n)
	}

	if result.Bundle == "" {
		t.Fatalf("bundle path empty")
	}
	if err := assertBundleContains(result.Bundle, []string{"rows.jsonl", "chain.jsonl", "ed25519.pub", "manifest.json"}); err != nil {
		t.Fatalf("bundle: %v", err)
	}
}

func TestS4AuditExportUnknownUseCase(t *testing.T) {
	withTempCache(t)
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
		"s4", "audit-export",
		"--cred", "sandbox",
		"--use-case", "nonexistent",
		"--from", "2026-01-01",
		"--to", "2026-03-31",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown use case") {
		t.Fatalf("expected unknown-usecase error, got %v", err)
	}
}

func TestS4AuditExportMissingRange(t *testing.T) {
	withTempCache(t)
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
		"s4", "audit-export",
		"--cred", "sandbox",
		"--use-case", "sox-journal",
	})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error when --from/--to missing")
	}
}

func assertBundleContains(path string, want []string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		for _, w := range want {
			if strings.HasSuffix(hdr.Name, w) {
				found[w] = true
			}
		}
	}
	for _, w := range want {
		if !found[w] {
			return &missingFileErr{name: w}
		}
	}
	return nil
}

type missingFileErr struct{ name string }

func (e *missingFileErr) Error() string { return "missing in bundle: " + e.name }

func TestEnforceRetainLicense_FreeAllowedUnder30(t *testing.T) {
	for _, days := range []int{0, 1, 7, 30} {
		if err := enforceRetainLicense(days); err != nil {
			t.Errorf("retain=%d should be allowed free-tier, got %v", days, err)
		}
	}
}

func TestEnforceRetainLicense_FreeRejectedOver30(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := enforceRetainLicense(60); err == nil {
		t.Fatal("retain=60 without license should fail")
	}
}

func TestEnforceRetainLicense_RejectOverHardMax(t *testing.T) {
	if err := enforceRetainLicense(400); err == nil {
		t.Fatal("retain=400 should hit hard 365-day cap")
	}
}
