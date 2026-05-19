// Package config loads runtime settings from a single, central .env file
// living at $XDG_CONFIG_HOME/sapctl/.env (default ~/.config/sapctl/.env).
//
// Precedence for any variable:
//   1. Already-set process environment       (highest)
//   2. Value from ~/.config/sapctl/.env       (only if not set above)
//
// We never overwrite an existing env value -- callers shelling out can
// pass overrides explicitly. The .env file format is a strict subset of
// the shell convention: one `KEY=VALUE` per line, comments with `#`,
// blank lines ignored, no shell expansion, optional double quotes
// stripped around VALUE.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultEnvPath returns the conventional file path.
func DefaultEnvPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sapctl", ".env"), nil
}

// LoadDefault is the boot-time entry point: reads DefaultEnvPath() if it
// exists, sets only keys that aren't already in os.Environ. Missing file
// is not an error.
func LoadDefault() error {
	p, err := DefaultEnvPath()
	if err != nil {
		return err
	}
	return Load(p)
}

// Load reads path and applies entries to the process environment with the
// precedence rule documented in the package comment.
func Load(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open .env: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := parseLine(line)
		if !ok {
			return fmt.Errorf(".env line %d: not a KEY=VALUE", lineNo)
		}
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf(".env line %d: set %s: %w", lineNo, key, err)
		}
	}
	return scanner.Err()
}

func parseLine(line string) (string, string, bool) {
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}
	idx := strings.Index(line, "=")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	if !validKey(key) {
		return "", "", false
	}
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	return key, val, true
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r >= 'A' && r <= 'Z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
