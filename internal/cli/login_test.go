package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/auth"
	"github.com/jmbataller/markist/cli/internal/config"
)

// stubOpenBrowser replaces the openBrowser seam for the duration of the
// test, restoring it on cleanup, and reports the URL it was called with.
func stubOpenBrowser(t *testing.T) *string {
	t.Helper()
	var got string
	orig := openBrowser
	openBrowser = func(u string) error {
		got = u
		return nil
	}
	t.Cleanup(func() { openBrowser = orig })
	return &got
}

func stubConfirmAgain(t *testing.T, fn func(string) (bool, error)) {
	t.Helper()
	orig := confirmAgain
	confirmAgain = fn
	t.Cleanup(func() { confirmAgain = orig })
}

func TestLoginFreshDeviceFlowSucceeds(t *testing.T) {
	withConfigDir(t)

	var tokenAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			var body struct {
				ClientName string `json:"client_name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.HasPrefix(body.ClientName, "markist-cli on ") {
				t.Errorf("client_name = %q", body.ClientName)
			}
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode:              "devcode123",
				UserCode:                "WDJB-MJHT",
				VerificationURI:         "https://markist.test/device",
				VerificationURIComplete: "https://markist.test/device?code=WDJB-MJHT",
				ExpiresIn:               60,
				Interval:                0,
			})
		case "/api/v1/auth/device/token":
			n := atomic.AddInt32(&tokenAttempts, 1)
			if n < 2 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(auth.TokenErrorBody{Error: auth.TokenErrorAuthorizationPending})
				return
			}
			_ = json.NewEncoder(w).Encode(auth.TokenResponse{
				AccessToken:      "mka_new",
				RefreshToken:     "mkr_new",
				TokenType:        "Bearer",
				ExpiresIn:        3600,
				RefreshExpiresIn: 7776000,
			})
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	openedURL := stubOpenBrowser(t)

	cmd := newLoginCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login RunE error: %v", err)
	}

	if !strings.Contains(out.String(), "Logged in as @jose") {
		t.Fatalf("output = %q, want it to contain Logged in as @jose", out.String())
	}
	if !strings.Contains(out.String(), "WDJB-MJHT") {
		t.Fatalf("output = %q, want it to contain the user code", out.String())
	}
	if *openedURL != "https://markist.test/device?code=WDJB-MJHT" {
		t.Fatalf("openBrowser called with %q", *openedURL)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Auth.AccessToken != "mka_new" || cfg.Auth.RefreshToken != "mkr_new" {
		t.Fatalf("saved auth = %+v", cfg.Auth)
	}
	if cfg.Auth.ClientName == "" {
		t.Fatalf("saved client name is empty")
	}
}

func TestLoginNoBrowserFlagSkipsOpening(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "devcode1", UserCode: "AAAA-BBBB",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=AAAA-BBBB",
				ExpiresIn: 60, Interval: 0,
			})
		case "/api/v1/auth/device/token":
			_ = json.NewEncoder(w).Encode(auth.TokenResponse{
				AccessToken: "mka_new", RefreshToken: "mkr_new", TokenType: "Bearer",
				ExpiresIn: 3600, RefreshExpiresIn: 7776000,
			})
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	orig := openBrowser
	called := false
	openBrowser = func(string) error { called = true; return nil }
	t.Cleanup(func() { openBrowser = orig })

	cmd := newLoginCommand()
	cmd.SetArgs([]string{"--no-browser"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login RunE error: %v", err)
	}
	if called {
		t.Fatal("openBrowser was called despite --no-browser")
	}
	if !strings.Contains(out.String(), "https://x/device?code=AAAA-BBBB") {
		t.Fatalf("output = %q, want it to print the verification URL", out.String())
	}
}

func TestLoginAlreadyLoggedInPromptDeclines(t *testing.T) {
	withConfigDir(t)

	var deviceCodeCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
		case "/api/v1/auth/device/code":
			atomic.AddInt32(&deviceCodeCalls, 1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)

	var promptedTitle string
	stubConfirmAgain(t, func(title string) (bool, error) {
		promptedTitle = title
		return false, nil
	})

	cmd := newLoginCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login RunE error: %v", err)
	}

	if !strings.Contains(promptedTitle, "Already logged in as @jose") {
		t.Fatalf("confirm title = %q", promptedTitle)
	}
	if got := atomic.LoadInt32(&deviceCodeCalls); got != 0 {
		t.Fatalf("device/code called %d times, want 0", got)
	}
	if !strings.Contains(out.String(), "Staying logged in as @jose") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLoginAlreadyLoggedInPromptAccepts(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "devcode2", UserCode: "CCCC-DDDD",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=CCCC-DDDD",
				ExpiresIn: 60, Interval: 0,
			})
		case "/api/v1/auth/device/token":
			_ = json.NewEncoder(w).Encode(auth.TokenResponse{
				AccessToken: "mka_new2", RefreshToken: "mkr_new2", TokenType: "Bearer",
				ExpiresIn: 3600, RefreshExpiresIn: 7776000,
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)
	stubOpenBrowser(t)
	stubConfirmAgain(t, func(string) (bool, error) { return true, nil })

	cmd := newLoginCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login RunE error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Auth.AccessToken != "mka_new2" {
		t.Fatalf("saved auth = %+v, want the new device-flow token", cfg.Auth)
	}
}

func TestLoginForceSkipsPrompt(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "devcode3", UserCode: "EEEE-FFFF",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=EEEE-FFFF",
				ExpiresIn: 60, Interval: 0,
			})
		case "/api/v1/auth/device/token":
			_ = json.NewEncoder(w).Encode(auth.TokenResponse{
				AccessToken: "mka_new3", RefreshToken: "mkr_new3", TokenType: "Bearer",
				ExpiresIn: 3600, RefreshExpiresIn: 7776000,
			})
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(api.MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	saveTestAuth(t, server.URL)
	stubOpenBrowser(t)
	stubConfirmAgain(t, func(string) (bool, error) {
		t.Fatal("confirmAgain should not be called with --force")
		return false, nil
	})

	cmd := newLoginCommand()
	cmd.SetArgs([]string{"--force"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login RunE error: %v", err)
	}
}

func TestLoginPollAccessDenied(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "d1", UserCode: "AAAA-BBBB",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=AAAA-BBBB",
				ExpiresIn: 60, Interval: 0,
			})
		case "/api/v1/auth/device/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(auth.TokenErrorBody{Error: auth.TokenErrorAccessDenied})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	stubOpenBrowser(t)

	cmd := newLoginCommand()
	cmd.SetArgs([]string{"--no-browser"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, auth.ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
}

func TestLoginPollExpiresGivesUpCleanly(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "d1", UserCode: "AAAA-BBBB",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=AAAA-BBBB",
				// Already expired by the time polling starts.
				ExpiresIn: 0, Interval: 5,
			})
		case "/api/v1/auth/device/token":
			t.Errorf("device/token should not be called once the code has already expired")
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	stubOpenBrowser(t)

	cmd := newLoginCommand()
	cmd.SetArgs([]string{"--no-browser"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, auth.ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestLoginCtrlCCancelsCleanlyWithNoConfigWrite(t *testing.T) {
	dir := withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(auth.DeviceCodeResponse{
				DeviceCode: "d1", UserCode: "AAAA-BBBB",
				VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?code=AAAA-BBBB",
				ExpiresIn: 60, Interval: 30, // long interval: must be interrupted, not waited out
			})
		case "/api/v1/auth/device/token":
			// If this is reached before cancellation (slow CI), keep it
			// pending so the test can never spuriously succeed.
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(auth.TokenErrorBody{Error: auth.TokenErrorAuthorizationPending})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)

	stubOpenBrowser(t)

	cmd := newLoginCommand()
	cmd.SetArgs([]string{"--no-browser"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := cmd.ExecuteContext(ctx)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("err = %v, want a cancellation error", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("took %v, want a prompt cancellation (interval was 30s, must not be waited out)", elapsed)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "config")); !os.IsNotExist(statErr) {
		t.Fatalf("config file exists after a canceled login (stat err = %v), want no partial write", statErr)
	}
}
