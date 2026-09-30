package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmbataller/markist/cli/internal/config"
)

func TestLogoutNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newLogoutCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("logout RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "Not logged in.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLogoutRevokesAndClearsTokensKeepingAPIURL(t *testing.T) {
	withConfigDir(t)

	var revokeCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/revoke" {
			t.Errorf("path = %s, want /api/v1/auth/revoke", r.URL.Path)
		}
		revokeCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	cfg.APIURL = "https://custom.test"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error: %v", err)
	}

	cmd := newLogoutCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("logout RunE error: %v", err)
	}
	if !revokeCalled {
		t.Fatal("server never received POST /api/v1/auth/revoke")
	}
	if !strings.Contains(out.String(), "Logged out.") {
		t.Fatalf("output = %q", out.String())
	}

	onDisk, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if onDisk.Auth != (config.Auth{}) {
		t.Fatalf("on-disk auth = %+v, want cleared", onDisk.Auth)
	}
	if onDisk.APIURL != "https://custom.test" {
		t.Fatalf("APIURL = %q, want it preserved", onDisk.APIURL)
	}
}

func TestLogoutRevokeFailureIsWarningNotFatal(t *testing.T) {
	withConfigDir(t)

	// A server that's already closed guarantees a fast connection-refused
	// error, standing in for "offline" without a real network dependency
	// or a hanging test.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)

	cmd := newLogoutCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("logout RunE error: %v, want nil (revoke failure is a warning)", err)
	}
	if !strings.Contains(out.String(), "Warning:") {
		t.Fatalf("output = %q, want a warning about the failed revoke", out.String())
	}
	if !strings.Contains(out.String(), "Logged out.") {
		t.Fatalf("output = %q, want local credentials still cleared", out.String())
	}

	onDisk, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if onDisk.Auth != (config.Auth{}) {
		t.Fatalf("on-disk auth = %+v, want cleared even though revoke failed", onDisk.Auth)
	}
}
