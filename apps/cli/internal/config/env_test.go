package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBasic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte(`
# comment line
FOO=bar
export BAZ="qux quux"
SAPCTL_AUDIT=1
`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"FOO", "BAZ", "SAPCTL_AUDIT"} {
		_ = os.Unsetenv(k)
	}
	if err := Load(p); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if os.Getenv("FOO") != "bar" {
		t.Errorf("FOO=%q", os.Getenv("FOO"))
	}
	if os.Getenv("BAZ") != "qux quux" {
		t.Errorf("BAZ=%q", os.Getenv("BAZ"))
	}
	if os.Getenv("SAPCTL_AUDIT") != "1" {
		t.Errorf("SAPCTL_AUDIT=%q", os.Getenv("SAPCTL_AUDIT"))
	}
}

func TestLoadDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=from_file\n"), 0o600)
	t.Setenv("FOO", "from_env")
	if err := Load(p); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := os.Getenv("FOO"); got != "from_env" {
		t.Fatalf("env was overwritten: %q", got)
	}
}

func TestLoadMissingFileNoError(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
}

func TestLoadRejectsBadLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("not-a-kv\n"), 0o600)
	if err := Load(p); err == nil {
		t.Fatalf("expected error on malformed line")
	}
}

func TestValidKey(t *testing.T) {
	for _, k := range []string{"FOO", "FOO_BAR", "X_1"} {
		if !validKey(k) {
			t.Errorf("%q should be valid", k)
		}
	}
	for _, k := range []string{"", "1FOO", "foo", "FOO-BAR", "FOO BAR"} {
		if validKey(k) {
			t.Errorf("%q should be invalid", k)
		}
	}
}
