package cli

import (
	"testing"

	"github.com/jmbataller/markist/cli/internal/config"
)

// withConfigDir points internal/config at a fresh temp directory for the
// duration of the test, shared by login/logout/whoami tests.
func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	return dir
}
