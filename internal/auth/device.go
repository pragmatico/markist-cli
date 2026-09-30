// Package auth implements the OAuth 2.0 Device Authorization Grant (RFC
// 8628) the CLI uses to obtain credentials: request a code, then poll for a
// token while the user approves it on /device. Both endpoints are
// unauthenticated (no bearer token exists yet), so this client is separate
// from internal/api.Client.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pragmatico/markist-cli/internal/api"
)

// DeviceCodeResponse mirrors the response body of
// POST /api/v1/auth/device/code (plan Task 5).
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse mirrors the success body shared by
// POST /api/v1/auth/device/token and POST /api/v1/auth/refresh.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// TokenErrorCode mirrors the RFC-shaped { "error": "..." } body the
// device/token and refresh endpoints return instead of api.ErrorBody --
// the one deliberate exception to the /api/v1 envelope (plan Task 5).
type TokenErrorCode string

const (
	TokenErrorAuthorizationPending TokenErrorCode = "authorization_pending"
	TokenErrorSlowDown             TokenErrorCode = "slow_down"
	TokenErrorAccessDenied         TokenErrorCode = "access_denied"
	TokenErrorExpiredToken         TokenErrorCode = "expired_token"
	TokenErrorInvalidGrant         TokenErrorCode = "invalid_grant"
)

// TokenErrorBody is the JSON shape of a device/token or refresh failure.
type TokenErrorBody struct {
	Error TokenErrorCode `json:"error"`
}

// ErrAccessDenied is returned by PollForToken when the user clicks Deny on
// /device.
var ErrAccessDenied = errors.New("login request was denied")

// ErrExpired is returned by PollForToken when the device code's expires_in
// deadline passes (locally, or because the server said expired_token)
// before the user approves the request.
var ErrExpired = errors.New("login request expired; run `markist login` again")

// defaultSlowDownIncrement is how much PollForToken lengthens its polling
// interval after a slow_down response, per RFC 8628's guidance of at least
// 5 seconds. Client.SlowDownIncrement overrides it (tests use a much
// smaller value to stay fast).
const defaultSlowDownIncrement = 5 * time.Second

// Client talks to the unauthenticated device-flow endpoints. Unlike
// api.Client it carries no bearer token -- these calls happen before the
// CLI has one.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client

	// SlowDownIncrement overrides defaultSlowDownIncrement; zero means use
	// the default.
	SlowDownIncrement time.Duration
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) slowDownIncrement() time.Duration {
	if c.SlowDownIncrement > 0 {
		return c.SlowDownIncrement
	}
	return defaultSlowDownIncrement
}

func setDeviceFlowHeaders(req *http.Request) {
	header := api.ClientHeaderValue()
	req.Header.Set("X-Markist-Client", header)
	req.Header.Set("User-Agent", header)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
}

// RequestDeviceCode starts the device flow by calling
// POST /api/v1/auth/device/code with the given client name (e.g.
// "markist-cli on <hostname>"). A non-2xx response decodes through the
// standard /api/v1 envelope (this endpoint isn't one of the RFC-shaped
// exceptions -- those are device/token and refresh).
func (c *Client) RequestDeviceCode(ctx context.Context, clientName string) (*DeviceCodeResponse, error) {
	body, err := json.Marshal(struct {
		ClientName string `json:"client_name"`
	}{ClientName: clientName})
	if err != nil {
		return nil, fmt.Errorf("encoding device code request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/auth/device/code", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building device code request: %w", err)
	}
	setDeviceFlowHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device code response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope api.ErrorBody
		if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error.Code != "" {
			return nil, &api.Error{StatusCode: resp.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
		return nil, &api.Error{StatusCode: resp.StatusCode, Code: api.ErrorCodeInternal, Message: string(data)}
	}

	var out DeviceCodeResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decoding device code response: %w", err)
	}
	return &out, nil
}

// requestToken makes one POST /api/v1/auth/device/token call. It returns
// exactly one of: a success body, a token error code, or an error.
func (c *Client) requestToken(ctx context.Context, deviceCode string) (*TokenResponse, TokenErrorCode, error) {
	body, err := json.Marshal(struct {
		GrantType  string `json:"grant_type"`
		DeviceCode string `json:"device_code"`
	}{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		DeviceCode: deviceCode,
	})
	if err != nil {
		return nil, "", fmt.Errorf("encoding device token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/auth/device/token", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("building device token request: %w", err)
	}
	setDeviceFlowHeaders(req)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("polling for token: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading device token response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out TokenResponse
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, "", fmt.Errorf("decoding device token response: %w", err)
		}
		return &out, "", nil
	}

	var envelope TokenErrorBody
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error != "" {
		return nil, envelope.Error, nil
	}
	return nil, "", fmt.Errorf("device token request failed with HTTP %d", resp.StatusCode)
}

// PollForToken polls POST /api/v1/auth/device/token at interval (honoring
// slow_down by lengthening it) until the device code is approved, denied,
// or its expiresIn deadline passes. ctx cancellation (e.g. Ctrl-C) returns
// promptly with ctx.Err() instead of waiting out the current interval.
func (c *Client) PollForToken(ctx context.Context, deviceCode string, interval, expiresIn time.Duration) (*TokenResponse, error) {
	deadline := time.Now().Add(expiresIn)

	for {
		if !time.Now().Before(deadline) {
			return nil, ErrExpired
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		resp, code, err := c.requestToken(ctx, deviceCode)
		if err != nil {
			return nil, err
		}
		if code == "" {
			return resp, nil
		}

		switch code {
		case TokenErrorAuthorizationPending:
			continue
		case TokenErrorSlowDown:
			interval += c.slowDownIncrement()
			continue
		case TokenErrorAccessDenied:
			return nil, ErrAccessDenied
		case TokenErrorExpiredToken:
			return nil, ErrExpired
		default:
			return nil, fmt.Errorf("device login failed: %s", code)
		}
	}
}
