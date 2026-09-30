package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".markist")
	t.Setenv(EnvConfigDir, configDir)
	return configDir
}

func TestDirHonoursEnvOverride(t *testing.T) {
	configDir := withConfigDir(t)
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	if dir != configDir {
		t.Fatalf("Dir() = %q, want %q", dir, configDir)
	}
}

func TestDirDefaultsToHomeMarkist(t *testing.T) {
	t.Setenv(EnvConfigDir, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	want := filepath.Join(home, ".markist")
	if dir != want {
		t.Fatalf("Dir() = %q, want %q", dir, want)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	withConfigDir(t)

	cfg := &Config{
		APIURL: "https://example.test",
		Auth: Auth{
			AccessToken:     "mka_abc",
			RefreshToken:    "mkr_def",
			AccessExpiresAt: "2026-01-01T00:00:00Z",
			ClientName:      "markist-cli on test-host",
		},
		Update: Update{
			LastCheckedAt: "2026-01-01T00:00:00Z",
			LatestSeen:    "0.2.0",
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if *got != *cfg {
		t.Fatalf("Load() = %+v, want %+v", *got, *cfg)
	}
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	withConfigDir(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if *got != (Config{}) {
		t.Fatalf("Load() = %+v, want zero value", *got)
	}
}

func TestSaveWritesDirAndFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file mode bits don't apply on windows")
	}
	configDir := withConfigDir(t)

	if err := Save(&Config{APIURL: "https://example.test"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	dirInfo, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir mode = %04o, want 0700", perm)
	}

	path := filepath.Join(configDir, "config")
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file mode = %04o, want 0600", perm)
	}
}

func TestSaveIsAtomicNoTempFilesLeftBehind(t *testing.T) {
	configDir := withConfigDir(t)

	if err := Save(&Config{APIURL: "https://example.test"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("read config dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "config" {
			t.Fatalf("unexpected leftover entry in config dir: %s", entry.Name())
		}
	}
}

func TestLoadRefusesGroupOrWorldReadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file mode bits don't apply on windows")
	}
	configDir := withConfigDir(t)

	if err := Save(&Config{APIURL: "https://example.test"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := filepath.Join(configDir, "config")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want a permission error")
	}
	if !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("Load() error = %q, want it to mention `chmod 600`", err.Error())
	}
}

func TestSaveOverwritesExistingFile(t *testing.T) {
	withConfigDir(t)

	if err := Save(&Config{APIURL: "https://one.test"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err := Save(&Config{APIURL: "https://two.test"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.APIURL != "https://two.test" {
		t.Fatalf("APIURL = %q, want %q", got.APIURL, "https://two.test")
	}
}

func TestPathIsInsideDir(t *testing.T) {
	configDir := withConfigDir(t)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if filepath.Dir(path) != configDir {
		t.Fatalf("Path() = %q, want it inside %q", path, configDir)
	}
}

func TestLockPathIsInsideDir(t *testing.T) {
	configDir := withConfigDir(t)

	path, err := LockPath()
	if err != nil {
		t.Fatalf("LockPath() error: %v", err)
	}
	if filepath.Dir(path) != configDir {
		t.Fatalf("LockPath() = %q, want it inside %q", path, configDir)
	}
}
