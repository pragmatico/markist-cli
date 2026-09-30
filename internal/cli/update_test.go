package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/config"
	"github.com/jmbataller/markist/cli/internal/update"
	"github.com/jmbataller/markist/cli/internal/version"
)

// stubCurrentVersion overrides version.Version (normally "dev" unless
// GoReleaser injected it via -ldflags), so Check has something parseable to
// compare the fetched release against.
func stubCurrentVersion(t *testing.T, v string) {
	t.Helper()
	orig := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = orig })
}

func TestShouldRunUpdateCheck(t *testing.T) {
	old := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	recent := time.Now().Format(time.RFC3339)

	tests := []struct {
		name        string
		cfg         *config.Config
		jsonFlag    bool
		interactive bool
		want        bool
	}{
		{"due and interactive", &config.Config{Update: config.Update{LastCheckedAt: old}}, false, true, true},
		{"never checked", &config.Config{}, false, true, true},
		{"checked recently", &config.Config{Update: config.Update{LastCheckedAt: recent}}, false, true, false},
		{"json flag skips", &config.Config{Update: config.Update{LastCheckedAt: old}}, true, true, false},
		{"non-interactive skips", &config.Config{Update: config.Update{LastCheckedAt: old}}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRunUpdateCheck(tt.cfg, tt.jsonFlag, tt.interactive); got != tt.want {
				t.Errorf("shouldRunUpdateCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRunUpdateCheckEnvOverrides(t *testing.T) {
	old := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	cfg := &config.Config{Update: config.Update{LastCheckedAt: old}}

	t.Run("MARKIST_NO_UPDATE_CHECK", func(t *testing.T) {
		t.Setenv("MARKIST_NO_UPDATE_CHECK", "1")
		if shouldRunUpdateCheck(cfg, false, true) {
			t.Error("expected the check to be skipped when MARKIST_NO_UPDATE_CHECK is set")
		}
	})

	t.Run("CI", func(t *testing.T) {
		t.Setenv("CI", "true")
		if shouldRunUpdateCheck(cfg, false, true) {
			t.Error("expected the check to be skipped when CI is set")
		}
	})
}

func TestPrintUpdateNotice(t *testing.T) {
	t.Run("nil result prints nothing", func(t *testing.T) {
		var out strings.Builder
		printUpdateNotice(&out, nil)
		if out.String() != "" {
			t.Errorf("output = %q, want empty", out.String())
		}
	})

	t.Run("no update prints nothing", func(t *testing.T) {
		var out strings.Builder
		printUpdateNotice(&out, &update.Result{CurrentVersion: "0.1.0", HasUpdate: false})
		if out.String() != "" {
			t.Errorf("output = %q, want empty", out.String())
		}
	})

	t.Run("update available prints a notice", func(t *testing.T) {
		var out strings.Builder
		printUpdateNotice(&out, &update.Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", HasUpdate: true})
		got := out.String()
		if !strings.Contains(got, "0.2.0") || !strings.Contains(got, "0.1.0") {
			t.Errorf("output = %q, want it to mention both versions", got)
		}
		if !strings.Contains(got, releasesURL) {
			t.Errorf("output = %q, want it to contain %s", got, releasesURL)
		}
	})
}

func stubUpdateSeams(t *testing.T, client update.HTTPDoer, apiBaseURL string, interactive bool) {
	t.Helper()
	origClient, origBaseURL, origInteractive := updateHTTPClient, updateAPIBaseURL, updateIsInteractive
	updateHTTPClient = client
	updateAPIBaseURL = apiBaseURL
	updateIsInteractive = func() bool { return interactive }
	t.Cleanup(func() {
		updateHTTPClient = origClient
		updateAPIBaseURL = origBaseURL
		updateIsInteractive = origInteractive
	})
}

func releaseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestStartUpdateCheckSkippedNonInteractive(t *testing.T) {
	withConfigDir(t)
	server := releaseServer(t, `[{"tag_name":"cli/v9.9.9"}]`)
	stubUpdateSeams(t, server.Client(), server.URL, false)

	result := <-startUpdateCheck(t.Context())
	if result != nil {
		t.Fatalf("startUpdateCheck() = %+v, want nil when non-interactive", result)
	}
}

func TestStartUpdateCheckFindsUpdate(t *testing.T) {
	withConfigDir(t)
	stubCurrentVersion(t, "0.1.0")
	server := releaseServer(t, `[{"tag_name":"cli/v9.9.9"}]`)
	stubUpdateSeams(t, server.Client(), server.URL, true)

	result := <-startUpdateCheck(t.Context())
	if result == nil || !result.HasUpdate || result.LatestVersion != "9.9.9" {
		t.Fatalf("startUpdateCheck() = %+v, want an available 9.9.9 update", result)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Update.LastCheckedAt == "" {
		t.Error("expected LastCheckedAt to be persisted")
	}
	if cfg.Update.LatestSeen != "9.9.9" {
		t.Errorf("LatestSeen = %q, want 9.9.9", cfg.Update.LatestSeen)
	}
}

func TestStartUpdateCheckRespectsCadence(t *testing.T) {
	withConfigDir(t)
	if err := config.Save(&config.Config{Update: config.Update{LastCheckedAt: time.Now().Format(time.RFC3339)}}); err != nil {
		t.Fatalf("config.Save() error: %v", err)
	}

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"cli/v9.9.9"}]`))
	}))
	t.Cleanup(server.Close)
	stubUpdateSeams(t, server.Client(), server.URL, true)

	result := <-startUpdateCheck(t.Context())
	if result != nil {
		t.Fatalf("startUpdateCheck() = %+v, want nil within the 24h cadence window", result)
	}
	if calls != 0 {
		t.Errorf("GitHub was called %d times, want 0 (cadence should have skipped it)", calls)
	}
}
