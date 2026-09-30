package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/config"
)

func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.EnvConfigDir, dir)
	return dir
}

func writeConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error: %v", err)
	}
}

// --- envelope error codes ---

func envelopeServer(t *testing.T, status int, code ErrorCode) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(ErrorBody{
			Error: struct {
				Code    ErrorCode `json:"code"`
				Message string    `json:"message"`
			}{Code: code, Message: "boom"},
		})
	}))
}

func TestMeDecodesEveryEnvelopeErrorCode(t *testing.T) {
	withConfigDir(t)

	cases := []struct {
		status int
		code   ErrorCode
	}{
		{http.StatusUnauthorized, ErrorCodeUnauthorized},
		{http.StatusForbidden, ErrorCodeOnboardingRequired},
		{http.StatusForbidden, ErrorCodeForbidden},
		{http.StatusTooManyRequests, ErrorCodeRateLimited},
		{http.StatusBadRequest, ErrorCodeValidationFailed},
		{http.StatusUpgradeRequired, ErrorCodeUpgradeRequired},
		{http.StatusNotFound, ErrorCodeNotFound},
		{http.StatusInternalServerError, ErrorCodeInternal},
	}

	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			server := envelopeServer(t, tc.status, tc.code)
			defer server.Close()

			// No refresh token: the client must not try to refresh on a
			// plain unauthorized/forbidden/etc response, it should just
			// surface the typed error from the single request.
			client := NewClient(server.URL, "mka_whatever")
			_, err := client.Me(context.Background())

			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("Me() error = %v, want *Error", err)
			}
			if apiErr.Code != tc.code {
				t.Fatalf("Code = %q, want %q", apiErr.Code, tc.code)
			}
			if apiErr.StatusCode != tc.status {
				t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
		})
	}
}

func TestMeSuccessDecodesBody(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Errorf("path = %s, want /api/v1/me", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mka_valid" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Markist-Client"); got == "" {
			t.Errorf("X-Markist-Client header missing")
		}
		_ = json.NewEncoder(w).Encode(MeDto{Username: "jose", Email: "jose@example.test"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "mka_valid")
	me, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if me.Username != "jose" || me.Email != "jose@example.test" {
		t.Fatalf("Me() = %+v", me)
	}
}

// --- retry-once on network error / 5xx for idempotent GETs, never on POST ---

func TestGetRetriesOnceOn5xxThenSucceeds(t *testing.T) {
	withConfigDir(t)

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(MeDto{Username: "jose", Email: "jose@example.test"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "mka_valid")
	me, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if me.Username != "jose" {
		t.Fatalf("Me() = %+v", me)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2 (one retry)", got)
	}
}

func TestGetDoesNotRetryTwice(t *testing.T) {
	withConfigDir(t)

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, "mka_valid")
	_, err := client.Me(context.Background())
	if err == nil {
		t.Fatal("Me() error = nil, want an error")
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2 (one retry, then give up)", got)
	}
}

func TestPostNeverRetriesOn5xx(t *testing.T) {
	withConfigDir(t)

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, "mka_valid")
	_, err := client.AddBookmark(context.Background(), AddBookmarkRequest{URL: "https://example.test"})
	if err == nil {
		t.Fatal("AddBookmark() error = nil, want an error")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry on POST)", got)
	}
}

// --- refresh-on-401 token_expired ---

func TestRefreshesOnTokenExpiredThenRetriesOriginalRequest(t *testing.T) {
	withConfigDir(t)
	writeConfig(t, &config.Config{
		Auth: config.Auth{
			AccessToken:     "mka_old",
			RefreshToken:    "mkr_old",
			AccessExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339), // not near expiry
			ClientName:      "test",
		},
	})

	var meAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/auth/refresh":
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.RefreshToken != "mkr_old" {
				t.Errorf("refresh_token = %q, want mkr_old", body.RefreshToken)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":       "mka_new",
				"refresh_token":      "mkr_new",
				"token_type":         "Bearer",
				"expires_in":         3600,
				"refresh_expires_in": 7776000,
			})
		case r.URL.Path == "/api/v1/me":
			n := atomic.AddInt32(&meAttempts, 1)
			if n == 1 {
				// First attempt: server says the (in-memory, still valid
				// per its own expiry) access token is actually expired.
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(ErrorBody{Error: struct {
					Code    ErrorCode `json:"code"`
					Message string    `json:"message"`
				}{Code: ErrorCodeTokenExpired, Message: "expired"}})
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer mka_new" {
				t.Errorf("second attempt Authorization = %q, want Bearer mka_new", got)
			}
			_ = json.NewEncoder(w).Encode(MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	client := NewClientFromConfig(server.URL, cfg)

	me, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if me.Username != "jose" {
		t.Fatalf("Me() = %+v", me)
	}
	if got := atomic.LoadInt32(&meAttempts); got != 2 {
		t.Fatalf("me attempts = %d, want 2", got)
	}

	onDisk, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if onDisk.Auth.AccessToken != "mka_new" || onDisk.Auth.RefreshToken != "mkr_new" {
		t.Fatalf("on-disk auth = %+v, want refreshed tokens", onDisk.Auth)
	}
}

func TestProactiveRefreshWhenAccessTokenNearExpiry(t *testing.T) {
	withConfigDir(t)
	writeConfig(t, &config.Config{
		Auth: config.Auth{
			AccessToken:     "mka_old",
			RefreshToken:    "mkr_old",
			AccessExpiresAt: time.Now().Add(10 * time.Second).Format(time.RFC3339), // inside the 60s skew
			ClientName:      "test",
		},
	})

	var refreshCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":       "mka_new",
				"refresh_token":      "mkr_new",
				"token_type":         "Bearer",
				"expires_in":         3600,
				"refresh_expires_in": 7776000,
			})
		case "/api/v1/me":
			if got := r.Header.Get("Authorization"); got != "Bearer mka_new" {
				t.Errorf("Authorization = %q, want Bearer mka_new (proactive refresh)", got)
			}
			_ = json.NewEncoder(w).Encode(MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	client := NewClientFromConfig(server.URL, cfg)

	if _, err := client.Me(context.Background()); err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
}

// --- invalid_grant clears tokens and fails with ErrSessionExpired ---

func TestInvalidGrantClearsTokensAndReturnsSessionExpired(t *testing.T) {
	withConfigDir(t)
	writeConfig(t, &config.Config{
		Auth: config.Auth{
			AccessToken:     "mka_old",
			RefreshToken:    "mkr_revoked",
			AccessExpiresAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
			ClientName:      "test",
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/refresh" {
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer server.Close()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	client := NewClientFromConfig(server.URL, cfg)

	_, err = client.Me(context.Background())
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("Me() error = %v, want ErrSessionExpired", err)
	}

	onDisk, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if onDisk.Auth != (config.Auth{}) {
		t.Fatalf("on-disk auth = %+v, want cleared", onDisk.Auth)
	}
}

// --- the refresh race: two clients sharing one config dir must not both
// present the stale refresh token to the server. ---

func TestConcurrentRefreshOnlyHitsServerOnce(t *testing.T) {
	withConfigDir(t)
	writeConfig(t, &config.Config{
		Auth: config.Auth{
			AccessToken:     "mka_old",
			RefreshToken:    "mkr_stale",
			AccessExpiresAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
			ClientName:      "test",
		},
	})

	var (
		mu            sync.Mutex
		refreshCalls  int
		currentAccess = "mka_old"
		currentRefr   = "mkr_stale"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)

			mu.Lock()
			defer mu.Unlock()
			refreshCalls++
			if body.RefreshToken != currentRefr {
				// Reuse of an already-rotated refresh token: exactly the
				// failure mode two racing processes must not trigger.
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			currentAccess = fmt.Sprintf("mka_new_%d", refreshCalls)
			currentRefr = fmt.Sprintf("mkr_new_%d", refreshCalls)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":       currentAccess,
				"refresh_token":      currentRefr,
				"token_type":         "Bearer",
				"expires_in":         3600,
				"refresh_expires_in": 7776000,
			})
		case "/api/v1/me":
			mu.Lock()
			ok := r.Header.Get("Authorization") == "Bearer "+currentAccess
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(ErrorBody{Error: struct {
					Code    ErrorCode `json:"code"`
					Message string    `json:"message"`
				}{Code: ErrorCodeTokenExpired, Message: "expired"}})
				return
			}
			_ = json.NewEncoder(w).Encode(MeDto{Username: "jose", Email: "jose@example.test"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	// Two independent clients, each loading the same on-disk config before
	// either starts refreshing -- simulating two concurrent `markist`
	// processes both holding the original stale refresh token in memory.
	cfgA, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	cfgB, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	clientA := NewClientFromConfig(server.URL, cfgA)
	clientB := NewClientFromConfig(server.URL, cfgB)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = clientA.Me(context.Background())
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = clientB.Me(context.Background())
	}()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("client %d Me() error: %v", i, err)
		}
	}

	mu.Lock()
	calls := refreshCalls
	mu.Unlock()
	if calls != 1 {
		t.Fatalf("refresh calls reaching the server = %d, want exactly 1", calls)
	}

	onDisk, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if onDisk.Auth.RefreshToken == "mkr_stale" {
		t.Fatal("on-disk refresh token was never updated")
	}
}

// --- API URL is otherwise unaffected by refresh plumbing ---

func TestNewClientDefaultsBaseURL(t *testing.T) {
	c := NewClient("", "mka_x")
	if c.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, DefaultBaseURL)
	}
}
