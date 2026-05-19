package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	// Reset global flag state between tests.
	globalFlags = GlobalFlags{}
	root := NewRootCmd("0.0.0-test")
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestVersionPlain(t *testing.T) {
	out, _, err := runCmd(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if strings.TrimSpace(out) != "0.0.0-test" {
		t.Fatalf("plain version output = %q", out)
	}
}

func TestVersionJSON(t *testing.T) {
	out, _, err := runCmd(t, "version", "--json")
	if err != nil {
		t.Fatalf("version --json: %v", err)
	}
	var got map[string]string
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, out)
	}
	if got["version"] != "0.0.0-test" {
		t.Fatalf("version field = %q", got["version"])
	}
	for _, k := range []string{"go_version", "os", "arch"} {
		if got[k] == "" {
			t.Errorf("missing field %q", k)
		}
	}
}

func TestVersionJSONCompact(t *testing.T) {
	out, _, err := runCmd(t, "version", "--json", "--compact")
	if err != nil {
		t.Fatalf("version --json --compact: %v", err)
	}
	if strings.Contains(out, "  ") {
		t.Fatalf("compact JSON should not contain double-space indent: %q", out)
	}
}

func TestGlobalFlagsRegistered(t *testing.T) {
	root := NewRootCmd("test")
	want := []string{"json", "select", "dry-run", "compact", "quiet", "yes", "no-input", "agent", "since"}
	for _, name := range want {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("persistent flag --%s missing", name)
		}
	}
}

func TestHelpExitsZero(t *testing.T) {
	_, _, err := runCmd(t, "--help")
	if err != nil {
		t.Fatalf("--help should not error: %v", err)
	}
}
