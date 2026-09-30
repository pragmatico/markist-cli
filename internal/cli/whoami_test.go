package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/config"
)

func TestWhoamiNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newWhoamiCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
	if exitCodeForError(err) != ExitNotLoggedIn {
		t.Fatalf("exitCodeForError(err) = %d, want %d", exitCodeForError(err), ExitNotLoggedIn)
	}
}

func TestWhoamiPlainOutput(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Errorf("path = %s, want /api/v1/me", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)

	cmd := newWhoamiCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("whoami RunE error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "@jose (jose@example.test)") {
		t.Fatalf("output = %q, want it to contain @jose (jose@example.test)", got)
	}
	if !strings.Contains(got, server.URL) {
		t.Fatalf("output = %q, want it to contain the API host %q", got, server.URL)
	}
}

func TestWhoamiJSONOutput(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newWhoamiCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("whoami RunE error: %v", err)
	}

	var got api.MeDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if got.Username != "jose" || got.Email != "jose@example.test" {
		t.Fatalf("got = %+v", got)
	}
}

// saveTestAuth writes a config with an access token that won't expire for
// an hour, so whoami/logout tests don't trip the client's proactive-refresh
// path (a blank/expired AccessExpiresAt would send every call through
// /api/v1/auth/refresh first).
func saveTestAuth(t *testing.T, apiURL string) {
	t.Helper()
	if err := config.Save(&config.Config{
		Auth: config.Auth{
			AccessToken:     "mka_valid",
			RefreshToken:    "mkr_valid",
			AccessExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
			ClientName:      "markist-cli on test-host",
		},
	}); err != nil {
		t.Fatalf("config.Save() error: %v", err)
	}
}
