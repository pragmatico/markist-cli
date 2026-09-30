package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/pragmatico/markist-cli/internal/config"
	"github.com/pragmatico/markist-cli/internal/version"
)

// DefaultBaseURL is the fallback when no --api-url flag, MARKIST_API_URL
// env var or config file value is set (internal/cli's resolveAPIURL).
const DefaultBaseURL = "https://markist.xyz"

// refreshSkew is how far ahead of the real access-token expiry the client
// proactively refreshes, so a request built just before expiry doesn't race
// the server's own clock.
const refreshSkew = 60 * time.Second

// Client is a thin HTTP client for /api/v1: typed methods, refresh-on-401,
// proactive refresh near expiry, and retry-on-5xx for idempotent GETs.
type Client struct {
	BaseURL         string
	AccessToken     string
	RefreshToken    string
	AccessExpiresAt time.Time
	ClientName      string
	HTTPClient      *http.Client
	Debug           bool

	// Now stands in for time.Now in tests; nil means time.Now.
	Now func() time.Time
}

// NewClient builds a client for baseURL (DefaultBaseURL if empty) using the
// given bearer access token. It has no refresh token, so it never attempts
// a refresh -- a 401 is simply returned to the caller. Use
// NewClientFromConfig for a client that can refresh itself.
func NewClient(baseURL, accessToken string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:     baseURL,
		AccessToken: accessToken,
		HTTPClient:  http.DefaultClient,
	}
}

// NewClientFromConfig builds a client from a loaded config, wiring up the
// refresh and access-expiry tracking that config.Auth carries.
func NewClientFromConfig(baseURL string, cfg *config.Config) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	c := &Client{
		BaseURL:      baseURL,
		AccessToken:  cfg.Auth.AccessToken,
		RefreshToken: cfg.Auth.RefreshToken,
		ClientName:   cfg.Auth.ClientName,
		HTTPClient:   http.DefaultClient,
	}
	if t, err := time.Parse(time.RFC3339, cfg.Auth.AccessExpiresAt); err == nil {
		c.AccessExpiresAt = t
	}
	return c
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// ClientHeaderValue is shared by X-Markist-Client and User-Agent, per the
// plan's Task 9 header requirement. Also reused by internal/auth's
// unauthenticated device-flow client, which sends the same header before it
// has a bearer token.
func ClientHeaderValue() string {
	return fmt.Sprintf("markist-cli/%s (%s; %s)", version.Version, runtime.GOOS, runtime.GOARCH)
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	header := ClientHeaderValue()
	req.Header.Set("X-Markist-Client", header)
	req.Header.Set("User-Agent", header)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}

	return req, nil
}

// do executes req, decoding a 2xx body into out (when out is non-nil) or a
// non-2xx body into a *Error.
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope ErrorBody
		if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error.Code != "" {
			return &Error{StatusCode: resp.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
		return &Error{StatusCode: resp.StatusCode, Code: ErrorCodeInternal, Message: string(data)}
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding response body: %w", err)
	}
	return nil
}

func (c *Client) doOnce(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

// isRetryable reports whether err is a transport-level failure or a 5xx
// response -- the two cases plan Task 10 allows a single retry for.
func isRetryable(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	return true
}

// doWithAuth is the entry point every typed method uses: it proactively
// refreshes a near-expiry access token, refreshes once and retries on a
// 401 token_expired response, and retries once more on a network error or
// 5xx -- but only for GET, since retrying a POST could double-submit.
func (c *Client) doWithAuth(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if err := c.ensureFreshToken(ctx); err != nil {
		return err
	}

	err := c.doOnce(ctx, method, path, query, body, out)

	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Code == ErrorCodeTokenExpired && c.RefreshToken != "" {
		if rerr := c.refreshWithLock(ctx); rerr != nil {
			return rerr
		}
		return c.doOnce(ctx, method, path, query, body, out)
	}

	if err != nil && method == http.MethodGet && isRetryable(err) {
		return c.doOnce(ctx, method, path, query, body, out)
	}

	return err
}

// Me calls GET /api/v1/me.
func (c *Client) Me(ctx context.Context) (*MeDto, error) {
	var out MeDto
	if err := c.doWithAuth(ctx, http.MethodGet, "/api/v1/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBookmarksParams are the query params for GET /api/v1/bookmarks.
type ListBookmarksParams struct {
	Query     string
	ReadLater bool
	Limit     int
}

// ListBookmarks calls GET /api/v1/bookmarks.
func (c *Client) ListBookmarks(ctx context.Context, params ListBookmarksParams) (*BookmarkListDto, error) {
	query := url.Values{}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.ReadLater {
		query.Set("readLater", "true")
	}
	if params.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", params.Limit))
	}

	var out BookmarkListDto
	if err := c.doWithAuth(ctx, http.MethodGet, "/api/v1/bookmarks", query, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddBookmarkRequest is the body for POST /api/v1/bookmarks.
type AddBookmarkRequest struct {
	URL       string   `json:"url"`
	ReadLater bool     `json:"readLater"`
	Tags      []string `json:"tags,omitempty"`
}

// AddBookmarkResponse is the 201 body for POST /api/v1/bookmarks.
type AddBookmarkResponse struct {
	ID string `json:"id"`
}

// AddBookmark calls POST /api/v1/bookmarks.
func (c *Client) AddBookmark(ctx context.Context, in AddBookmarkRequest) (*AddBookmarkResponse, error) {
	var out AddBookmarkResponse
	if err := c.doWithAuth(ctx, http.MethodPost, "/api/v1/bookmarks", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Shared calls GET /api/v1/shared.
func (c *Client) Shared(ctx context.Context) (*SharedListDto, error) {
	var out SharedListDto
	if err := c.doWithAuth(ctx, http.MethodGet, "/api/v1/shared", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke calls POST /api/v1/auth/revoke for the caller's own session.
func (c *Client) Revoke(ctx context.Context) error {
	return c.doWithAuth(ctx, http.MethodPost, "/api/v1/auth/revoke", nil, nil, nil)
}
