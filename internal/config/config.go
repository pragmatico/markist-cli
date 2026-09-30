// Package config manages ~/.markist/config (TOML): the API URL override,
// saved auth tokens, and update-check bookkeeping.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
)

// EnvConfigDir overrides the config directory location (dev/testing).
const EnvConfigDir = "MARKIST_CONFIG_DIR"

// EnvAPIURL overrides the API base URL, ranking above the config file but
// below the --api-url flag.
const EnvAPIURL = "MARKIST_API_URL"

// Auth holds the saved device-flow token pair and the client name they were
// issued under.
type Auth struct {
	AccessToken     string `toml:"access_token,omitempty"`
	RefreshToken    string `toml:"refresh_token,omitempty"`
	AccessExpiresAt string `toml:"access_expires_at,omitempty"`
	ClientName      string `toml:"client_name,omitempty"`
}

// Update tracks the daily release-check bookkeeping (plan Task 14).
type Update struct {
	LastCheckedAt string `toml:"last_checked_at,omitempty"`
	LatestSeen    string `toml:"latest_seen,omitempty"`
}

// Config is the full ~/.markist/config document.
type Config struct {
	APIURL string `toml:"api_url,omitempty"`
	Auth   Auth   `toml:"auth,omitempty"`
	Update Update `toml:"update,omitempty"`
}

// Dir resolves the config directory: MARKIST_CONFIG_DIR if set, otherwise
// ~/.markist (%USERPROFILE%\.markist on Windows, via os.UserHomeDir).
func Dir() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".markist"), nil
}

// Path returns the full path to the config file inside Dir().
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

// LockPath returns the path to the flock lockfile used to serialize token
// refreshes across concurrent processes (plan Task 10).
func LockPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.lock"), nil
}

// Load reads and parses the config file, returning an empty Config if it
// doesn't exist yet. On Unix, a config file readable or writable by group
// or others fails loudly rather than silently trusting file contents that
// could have been tampered with or read by another account.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			return nil, fmt.Errorf(
				"config file %s is readable or writable by group/others (mode %04o); fix it with: chmod 600 %s",
				path, perm, path,
			)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes cfg to disk atomically: it's marshaled into a temp file in
// the same directory, chmod'd to 0600, then renamed over the real path, so
// a crash or concurrent read never observes a partially written file.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}

	path, err := Path()
	if err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// NewLock returns the flock guarding concurrent token refreshes across
// markist processes (plan Task 10). Acquiring it is left to the caller.
func NewLock() (*flock.Flock, error) {
	path, err := LockPath()
	if err != nil {
		return nil, err
	}
	return flock.New(path), nil
}
