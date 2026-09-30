package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/api"
)

// --- RequestDeviceCode ---

func TestRequestDeviceCodeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/device/code" {
			t.Errorf("path = %s, want /api/v1/auth/device/code", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("X-Markist-Client"); got == "" {
			t.Errorf("X-Markist-Client header missing")
		}

		var body struct {
			ClientName string `json:"client_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ClientName != "markist-cli on test-host" {
			t.Errorf("client_name = %q, want %q", body.ClientName, "markist-cli on test-host")
		}

		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:              "devcode123",
			UserCode:                "WDJB-MJHT",
			VerificationURI:         "https://markist.test/device",
			VerificationURIComplete: "https://markist.test/device?code=WDJB-MJHT",
			ExpiresIn:               600,
			Interval:                5,
		})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	resp, err := client.RequestDeviceCode(context.Background(), "markist-cli on test-host")
	if err != nil {
		t.Fatalf("RequestDeviceCode() error: %v", err)
	}
	if resp.DeviceCode != "devcode123" || resp.UserCode != "WDJB-MJHT" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.ExpiresIn != 600 || resp.Interval != 5 {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestRequestDeviceCodeDecodesEnvelopeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(api.ErrorBody{
			Error: struct {
				Code    api.ErrorCode `json:"code"`
				Message string        `json:"message"`
			}{Code: api.ErrorCodeRateLimited, Message: "too many device code requests"},
		})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	_, err := client.RequestDeviceCode(context.Background(), "markist-cli on test-host")

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *api.Error", err)
	}
	if apiErr.Code != api.ErrorCodeRateLimited {
		t.Fatalf("Code = %q, want %q", apiErr.Code, api.ErrorCodeRateLimited)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusTooManyRequests)
	}
}

// --- PollForToken ---

func TestPollForTokenAuthorizationPendingThenSuccess(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/device/token" {
			t.Errorf("path = %s, want /api/v1/auth/device/token", r.URL.Path)
		}
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorAuthorizationPending})
			return
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:      "mka_x",
			RefreshToken:     "mkr_x",
			TokenType:        "Bearer",
			ExpiresIn:        3600,
			RefreshExpiresIn: 7776000,
		})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	resp, err := client.PollForToken(context.Background(), "devcode", time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("PollForToken() error: %v", err)
	}
	if resp.AccessToken != "mka_x" || resp.RefreshToken != "mkr_x" {
		t.Fatalf("resp = %+v", resp)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestPollForTokenSlowDownIncreasesInterval(t *testing.T) {
	var (
		mu         sync.Mutex
		attempts   int
		timestamps []time.Time
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		timestamps = append(timestamps, time.Now())
		mu.Unlock()

		switch {
		case n == 1:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorSlowDown})
		case n < 3:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorAuthorizationPending})
		default:
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "mka_x", RefreshToken: "mkr_x", TokenType: "Bearer",
				ExpiresIn: 3600, RefreshExpiresIn: 7776000,
			})
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, SlowDownIncrement: 20 * time.Millisecond}
	_, err := client.PollForToken(context.Background(), "devcode", time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("PollForToken() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(timestamps) < 3 {
		t.Fatalf("recorded %d requests, want at least 3", len(timestamps))
	}
	// The gap after the slow_down response (index 0 -> 1) must reflect the
	// lengthened interval; the gap before it (there is none, it's the
	// first request) can't be compared, so we only check post-slow_down.
	gap := timestamps[1].Sub(timestamps[0])
	if gap < 15*time.Millisecond {
		t.Fatalf("gap after slow_down = %v, want >= ~SlowDownIncrement (20ms)", gap)
	}
}

func TestPollForTokenAccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorAccessDenied})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	_, err := client.PollForToken(context.Background(), "devcode", time.Millisecond, time.Second)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
}

func TestPollForTokenExpiredTokenFromServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorExpiredToken})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	_, err := client.PollForToken(context.Background(), "devcode", time.Millisecond, time.Second)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestPollForTokenLocalDeadlineExpiresWithoutPolling(t *testing.T) {
	var called int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorAuthorizationPending})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	// expiresIn 0: the deadline has already passed before the first poll.
	_, err := client.PollForToken(context.Background(), "devcode", time.Second, 0)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
	if got := atomic.LoadInt32(&called); got != 0 {
		t.Fatalf("server called %d times, want 0 (already expired locally)", got)
	}
}

func TestPollForTokenContextCancelledReturnsPromptly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorAuthorizationPending})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	client := &Client{BaseURL: server.URL}
	start := time.Now()
	_, err := client.PollForToken(ctx, "devcode", 5*time.Second, time.Minute)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed > time.Second {
		t.Fatalf("took %v, want a prompt return (interval was 5s, must not be waited out)", elapsed)
	}
}

func TestPollForTokenUnknownErrorCodeIsFatal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(TokenErrorBody{Error: TokenErrorInvalidGrant})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL}
	_, err := client.PollForToken(context.Background(), "devcode", time.Millisecond, time.Second)
	if err == nil {
		t.Fatal("PollForToken() error = nil, want an error")
	}
	if errors.Is(err, ErrAccessDenied) || errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want a distinct fatal error for invalid_grant, not access-denied/expired", err)
	}
}
