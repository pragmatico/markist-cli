package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jmbataller/markist/cli/internal/config"
)

// ErrSessionExpired is returned when the refresh token itself has been
// rejected (invalid_grant): expired, revoked, or replayed by a racing
// process. internal/cli maps this to exit code 3 and the
// "Session expired or revoked. Run `markist login`." message.
var ErrSessionExpired = errors.New("session expired or revoked. Run `markist login`")

const refreshPath = "/api/v1/auth/refresh"

const tokenErrorInvalidGrant = "invalid_grant"

type refreshRequestBody struct {
	RefreshToken string `json:"refresh_token"`
}

// tokenSuccessBody mirrors the success body shared by
// POST /api/v1/auth/device/token and POST /api/v1/auth/refresh.
type tokenSuccessBody struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// tokenErrorBody is the RFC 6749 `{ "error": "..." }` shape the device/token
// and refresh endpoints use instead of the /api/v1 envelope.
type tokenErrorBody struct {
	Error string `json:"error"`
}

// ensureFreshToken refreshes the access token if it's within refreshSkew of
// expiry (or its expiry is unknown). A client with no refresh token (never
// logged in, or built with plain NewClient) is left untouched: the request
// proceeds and any 401 is returned to the caller as-is.
func (c *Client) ensureFreshToken(ctx context.Context) error {
	if c.RefreshToken == "" {
		return nil
	}
	if c.AccessToken != "" && !c.AccessExpiresAt.IsZero() && time.Until(c.AccessExpiresAt) > refreshSkew {
		return nil
	}
	return c.refreshWithLock(ctx)
}

// refreshWithLock serializes token refreshes across every markist process
// sharing this config directory. It takes an exclusive flock on
// config.lock, re-reads the config from disk, and only calls the server if
// the refresh token on disk still matches the one this client started
// with -- otherwise another process already refreshed while we waited for
// the lock, and we simply adopt its result. This is what stops two
// concurrent processes from both presenting the same stale refresh token,
// which would trip the server's reuse detection and revoke the session.
func (c *Client) refreshWithLock(ctx context.Context) error {
	if c.RefreshToken == "" {
		return ErrSessionExpired
	}

	lock, err := config.NewLock()
	if err != nil {
		return fmt.Errorf("opening config lock: %w", err)
	}
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("locking config: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	diskCfg, err := config.Load()
	if err != nil {
		return err
	}

	if diskCfg.Auth.RefreshToken != "" && diskCfg.Auth.RefreshToken != c.RefreshToken {
		c.adoptFromConfig(diskCfg)
		return nil
	}

	resp, tokenErr, err := c.requestRefresh(ctx, c.RefreshToken)
	if err != nil {
		return err
	}
	if tokenErr != "" {
		if tokenErr == tokenErrorInvalidGrant {
			diskCfg.Auth = config.Auth{}
			if saveErr := config.Save(diskCfg); saveErr != nil {
				return saveErr
			}
			c.AccessToken = ""
			c.RefreshToken = ""
			c.AccessExpiresAt = time.Time{}
			return ErrSessionExpired
		}
		return fmt.Errorf("refreshing session: unexpected %q response", tokenErr)
	}

	expiresAt := c.now().Add(time.Duration(resp.ExpiresIn) * time.Second)

	c.AccessToken = resp.AccessToken
	c.RefreshToken = resp.RefreshToken
	c.AccessExpiresAt = expiresAt

	diskCfg.Auth.AccessToken = resp.AccessToken
	diskCfg.Auth.RefreshToken = resp.RefreshToken
	diskCfg.Auth.AccessExpiresAt = expiresAt.Format(time.RFC3339)
	if diskCfg.Auth.ClientName == "" {
		diskCfg.Auth.ClientName = c.ClientName
	}
	return config.Save(diskCfg)
}

func (c *Client) adoptFromConfig(cfg *config.Config) {
	c.AccessToken = cfg.Auth.AccessToken
	c.RefreshToken = cfg.Auth.RefreshToken
	if t, err := time.Parse(time.RFC3339, cfg.Auth.AccessExpiresAt); err == nil {
		c.AccessExpiresAt = t
	}
}

// requestRefresh calls POST /api/v1/auth/refresh directly (not through
// doWithAuth, to avoid recursing into the refresh logic it's part of).
// It returns exactly one of: a success body, a token error code, or an
// error -- never more than one non-zero.
func (c *Client) requestRefresh(ctx context.Context, refreshToken string) (*tokenSuccessBody, string, error) {
	req, err := c.newRequest(ctx, http.MethodPost, refreshPath, nil, refreshRequestBody{RefreshToken: refreshToken})
	if err != nil {
		return nil, "", err
	}
	// The refresh endpoint authenticates via the body, not a bearer header.
	req.Header.Del("Authorization")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out tokenSuccessBody
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, "", fmt.Errorf("decoding refresh response: %w", err)
		}
		return &out, "", nil
	}

	var envelope tokenErrorBody
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error != "" {
		return nil, envelope.Error, nil
	}
	return nil, "", fmt.Errorf("refresh failed with HTTP %d", resp.StatusCode)
}
