package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBundleRoundTrip exports a small directory, verifies the bundle, then
// installs it and confirms the payload landed.
func TestBundleRoundTrip(t *testing.T) {
	src := t.TempDir()
	for _, p := range []string{"a.txt", "sub/b.txt"} {
		full := filepath.Join(src, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o700)
		if err := os.WriteFile(full, []byte("content of "+p), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	globalFlags = GlobalFlags{}
	root := NewRootCmd("test")
	var b1, e1 bytes.Buffer
	root.SetOut(&b1)
	root.SetErr(&e1)
	root.SetArgs([]string{
		"bundle", "export",
		"--name", "test-bundle",
		"--version", "9.9.9",
		"--dir", src,
		"--out", out,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("export: %v\nstderr=%s", err, e1.String())
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("bundle missing: %v", err)
	}

	globalFlags = GlobalFlags{}
	root2 := NewRootCmd("test")
	var b2, e2 bytes.Buffer
	root2.SetOut(&b2)
	root2.SetErr(&e2)
	root2.SetArgs([]string{"bundle", "verify", "--bundle", out})
	if err := root2.Execute(); err != nil {
		t.Fatalf("verify: %v\nstderr=%s", err, e2.String())
	}
	if !strings.Contains(b2.String(), "ok:") {
		t.Fatalf("verify out: %s", b2.String())
	}

	dest := t.TempDir()
	globalFlags = GlobalFlags{}
	root3 := NewRootCmd("test")
	var b3, e3 bytes.Buffer
	root3.SetOut(&b3)
	root3.SetErr(&e3)
	root3.SetArgs([]string{"bundle", "install", "--bundle", out, "--dest", dest})
	if err := root3.Execute(); err != nil {
		t.Fatalf("install: %v\nstderr=%s", err, e3.String())
	}

	got, err := os.ReadFile(filepath.Join(dest, filepath.Base(src), "a.txt"))
	if err != nil {
		t.Fatalf("missing installed a.txt: %v", err)
	}
	if !strings.Contains(string(got), "content of a.txt") {
		t.Fatalf("a.txt body wrong: %s", got)
	}
}

func TestBundleTamperFails(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("trusted content"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")
	globalFlags = GlobalFlags{}
	root := NewRootCmd("test")
	var b, e bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&e)
	root.SetArgs([]string{
		"bundle", "export",
		"--dir", src,
		"--out", out,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("export: %v\nstderr=%s", err, e.String())
	}

	tmp := t.TempDir()
	if err := extractTarGz(out, tmp); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "payload", filepath.Base(src), "file.txt")
	if err := os.WriteFile(target, []byte("TAMPERED content"), 0o600); err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.tar.gz")
	if err := writeTarGz(tmp, tampered); err != nil {
		t.Fatal(err)
	}

	globalFlags = GlobalFlags{}
	root2 := NewRootCmd("test")
	var b2, e2 bytes.Buffer
	root2.SetOut(&b2)
	root2.SetErr(&e2)
	root2.SetArgs([]string{"bundle", "verify", "--bundle", tampered})
	err := root2.Execute()
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("expected hash mismatch, got %v", err)
	}
}

func TestBundleExportEmptyFails(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bundle.tar.gz")
	globalFlags = GlobalFlags{}
	root := NewRootCmd("test")
	var b, e bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&e)
	root.SetArgs([]string{"bundle", "export", "--out", out})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "nothing to bundle") {
		t.Fatalf("expected empty bundle error, got %v", err)
	}
}

func TestBundleManifestPubKey(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "bundle.tar.gz")
	globalFlags = GlobalFlags{}
	root := NewRootCmd("test")
	root.SetArgs([]string{"bundle", "export", "--dir", src, "--out", out, "--json"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("export: %v", err)
	}
	tmp := t.TempDir()
	if err := extractTarGz(out, tmp); err != nil {
		t.Fatal(err)
	}
	mbytes, _ := os.ReadFile(filepath.Join(tmp, "bundle.json"))
	var m bundleManifest
	if err := json.Unmarshal(mbytes, &m); err != nil {
		t.Fatalf("manifest decode: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(m.PublicKeyB64)
	if err != nil || len(raw) != 32 {
		t.Fatalf("pub key not 32 bytes ed25519: len=%d err=%v", len(raw), err)
	}
}
