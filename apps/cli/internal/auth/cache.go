package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/dixitsheta/sapctl/apps/cli/internal/errs"
)

// Cache is the on-disk credential store at $XDG_CONFIG_HOME/sapctl/tokens.json
// (or ~/.config/sapctl/tokens.json). The file is created with 0600 perms.
//
// Cache stores long-lived credentials, not bearer tokens. Bearer tokens are
// kept only in memory (see XSUAA.fetchToken). Storing live bearer tokens on
// disk is an explicit non-goal: refresh is cheap and disk theft is a real
// threat model.
type Cache struct {
	path string
	mu   sync.Mutex
}

type cacheFile struct {
	Providers map[string]Config `json:"providers"`
}

// DefaultPath returns the conventional cache file path.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sapctl", "tokens.json"), nil
}

// NewCache opens (or creates) the cache at path.
func NewCache(path string) *Cache {
	return &Cache{path: path}
}

// Save writes cfg under label, overwriting any existing entry. Atomic via
// write-temp + rename. Label becomes cfg.Label.
func (c *Cache) Save(label string, cfg Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if label == "" {
		return errs.New(errs.ExitUserError, "auth.cache.label", "label is required")
	}

	cf, err := c.readLocked()
	if err != nil {
		return err
	}
	if cf.Providers == nil {
		cf.Providers = map[string]Config{}
	}
	cfg.Label = label
	cf.Providers[label] = cfg

	return c.writeLocked(cf)
}

// Load returns the config stored under label.
func (c *Cache) Load(label string) (Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cf, err := c.readLocked()
	if err != nil {
		return Config{}, err
	}
	cfg, ok := cf.Providers[label]
	if !ok {
		return Config{}, errs.New(errs.ExitNotFound, "auth.cache.not_found",
			fmt.Sprintf("no stored credential with label %q", label))
	}
	return cfg, nil
}

// Delete removes the entry. Missing is not an error.
func (c *Cache) Delete(label string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cf, err := c.readLocked()
	if err != nil {
		return err
	}
	delete(cf.Providers, label)
	return c.writeLocked(cf)
}

// List returns labels of stored credentials.
func (c *Cache) List() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cf, err := c.readLocked()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(cf.Providers))
	for k := range cf.Providers {
		out = append(out, k)
	}
	return out, nil
}

func (c *Cache) readLocked() (cacheFile, error) {
	b, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return cacheFile{Providers: map[string]Config{}}, nil
	}
	if err != nil {
		return cacheFile{}, errs.Wrap(errs.ExitUserError, "auth.cache.read", "read cache", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(b, &cf); err != nil {
		return cacheFile{}, errs.Wrap(errs.ExitUserError, "auth.cache.parse", "parse cache", err)
	}
	if cf.Providers == nil {
		cf.Providers = map[string]Config{}
	}
	return cf, nil
}

func (c *Cache) writeLocked(cf cacheFile) error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return errs.Wrap(errs.ExitUserError, "auth.cache.mkdir", "create cache dir", err)
	}
	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return errs.Wrap(errs.ExitUserError, "auth.cache.encode", "encode cache", err)
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errs.Wrap(errs.ExitUserError, "auth.cache.write_tmp", "write cache tmp", err)
	}
	if err := os.Rename(tmp, c.path); err != nil {
		_ = os.Remove(tmp)
		return errs.Wrap(errs.ExitUserError, "auth.cache.rename", "rename cache", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(c.path, 0o600)
	}
	return nil
}
