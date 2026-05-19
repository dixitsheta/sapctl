package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newTempCache(t *testing.T) (*Cache, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	return NewCache(path), path
}

func TestCacheRoundTrip(t *testing.T) {
	c, path := newTempCache(t)

	cfg := Config{Provider: "apikey", APIKey: "secret-abc123"}
	if err := c.Save("sandbox", cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := c.Load("sandbox")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != cfg.APIKey {
		t.Fatalf("APIKey mismatch: %q", got.APIKey)
	}
	if got.Label != "sandbox" {
		t.Fatalf("Label not set: %q", got.Label)
	}

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("cache file perms = %o, want 0600", fi.Mode().Perm())
		}
	}
}

func TestCacheLoadMissing(t *testing.T) {
	c, _ := newTempCache(t)
	_, err := c.Load("nope")
	if err == nil {
		t.Fatalf("expected error on missing label")
	}
}

func TestCacheDelete(t *testing.T) {
	c, _ := newTempCache(t)
	_ = c.Save("a", Config{Provider: "apikey", APIKey: "x"})
	_ = c.Save("b", Config{Provider: "apikey", APIKey: "y"})

	if err := c.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := c.Load("a"); err == nil {
		t.Fatalf("a should be gone")
	}
	if _, err := c.Load("b"); err != nil {
		t.Fatalf("b should remain: %v", err)
	}
}

func TestCacheList(t *testing.T) {
	c, _ := newTempCache(t)
	_ = c.Save("alpha", Config{Provider: "apikey", APIKey: "x"})
	_ = c.Save("beta", Config{Provider: "apikey", APIKey: "y"})

	labels, err := c.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("want 2 labels, got %d: %v", len(labels), labels)
	}
}
