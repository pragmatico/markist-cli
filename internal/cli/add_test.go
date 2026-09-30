package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAddNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"https://example.com/article"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
	if exitCodeForError(err) != ExitNotLoggedIn {
		t.Fatalf("exitCodeForError(err) = %d, want %d", exitCodeForError(err), ExitNotLoggedIn)
	}
}

func TestAddSingleURL(t *testing.T) {
	withConfigDir(t)

	var gotBody addRequestCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/bookmarks" {
			t.Errorf("path = %s, want /api/v1/bookmarks", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_1"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"https://example.com/article"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("add RunE error: %v", err)
	}

	want := "Saved: https://example.com/article — title and summary will appear in a few seconds\n"
	if out.String() != want {
		t.Fatalf("out = %q, want %q", out.String(), want)
	}
	if gotBody.URL != "https://example.com/article" {
		t.Fatalf("gotBody.URL = %q", gotBody.URL)
	}
	if gotBody.ReadLater {
		t.Fatalf("gotBody.ReadLater = true, want false")
	}
	if len(gotBody.Tags) != 0 {
		t.Fatalf("gotBody.Tags = %v, want empty", gotBody.Tags)
	}
}

func TestAddReadLaterAndTags(t *testing.T) {
	withConfigDir(t)

	var gotBody addRequestCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_2"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"https://example.com/article", "--read-later", "--tag", "rust", "--tag", "cli"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("add RunE error: %v", err)
	}

	if !gotBody.ReadLater {
		t.Fatalf("gotBody.ReadLater = false, want true")
	}
	if want := []string{"rust", "cli"}; !equalStrings(gotBody.Tags, want) {
		t.Fatalf("gotBody.Tags = %v, want %v", gotBody.Tags, want)
	}
}

func TestAddInvalidURLRejectedBeforeHTTPCall(t *testing.T) {
	withConfigDir(t)

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"not-a-url"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("add RunE error = nil, want an error")
	}
	if exitCodeForError(err) != ExitError {
		t.Fatalf("exitCodeForError(err) = %d, want %d", exitCodeForError(err), ExitError)
	}
	if called {
		t.Fatal("server was called for an invalid URL")
	}
	if !strings.Contains(out.String(), "invalid URL") {
		t.Fatalf("out = %q, want it to mention the invalid URL", out.String())
	}
}

func TestAddStdinMixedSuccessAndFailure(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body addRequestCapture
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.Contains(body.URL, "reject") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"code": "validation_failed", "message": "duplicate bookmark"},
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_" + body.URL})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{})
	cmd.SetIn(strings.NewReader("https://good.example/a\nhttps://reject.example/b\nhttps://good.example/c\n"))
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("add RunE error = nil, want an error because one line failed")
	}
	if exitCodeForError(err) != ExitError {
		t.Fatalf("exitCodeForError(err) = %d, want %d", exitCodeForError(err), ExitError)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d output lines, want 3: %q", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "Saved: https://good.example/a") {
		t.Fatalf("lines[0] = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "Failed: https://reject.example/b") {
		t.Fatalf("lines[1] = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "Saved: https://good.example/c") {
		t.Fatalf("lines[2] = %q", lines[2])
	}
}

func TestAddJSONSingleLine(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_json"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"https://example.com/article"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("add RunE error: %v", err)
	}

	var got struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	line := strings.TrimSpace(out.String())
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", line, err)
	}
	if got.ID != "bm_json" || got.URL != "https://example.com/article" {
		t.Fatalf("got = %+v", got)
	}
}

func TestAddJSONMultiLine(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body addRequestCapture
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.Contains(body.URL, "reject") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"code": "validation_failed", "message": "duplicate bookmark"},
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_ok"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{})
	cmd.SetIn(strings.NewReader("https://good.example/a\nhttps://reject.example/b\n"))
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("add RunE error = nil, want an error because one line failed")
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2: %q", len(lines), out.String())
	}

	var success struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &success); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", lines[0], err)
	}
	if success.ID != "bm_ok" || success.URL != "https://good.example/a" {
		t.Fatalf("success = %+v", success)
	}

	var failure struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &failure); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", lines[1], err)
	}
	if failure.URL != "https://reject.example/b" || failure.Error == "" {
		t.Fatalf("failure = %+v", failure)
	}
}

func TestAddOpenTriggersBrowserOnceForMultipleURLs(t *testing.T) {
	withConfigDir(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "bm_x"})
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	openedURLs := stubOpenBrowser(t)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"--open"})
	cmd.SetIn(strings.NewReader("https://good.example/a\nhttps://good.example/b\n"))
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("add RunE error: %v", err)
	}

	if *openedURLs != server.URL+"/dashboard" {
		t.Fatalf("openBrowser called with %q, want %q", *openedURLs, server.URL+"/dashboard")
	}
}

func TestAddOpenSkippedWhenNothingSucceeded(t *testing.T) {
	withConfigDir(t)
	saveTestAuth(t, "https://example.test")

	openedURLs := stubOpenBrowser(t)

	cmd := newTestAddCommand()
	cmd.SetArgs([]string{"not-a-url", "--open"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("add RunE error = nil, want an error")
	}
	if *openedURLs != "" {
		t.Fatalf("openBrowser called with %q, want it not to be called", *openedURLs)
	}
}

// addRequestCapture mirrors api.AddBookmarkRequest for decoding the test
// server's received body without importing api in this table (kept local
// to avoid a cyclic/needless dependency on the exact wire type).
type addRequestCapture struct {
	URL       string   `json:"url"`
	ReadLater bool     `json:"readLater"`
	Tags      []string `json:"tags"`
}

// newTestAddCommand builds an add command the way tests need it: called
// directly rather than through the root command, so it needs its own
// SilenceUsage/SilenceErrors (root.go sets these for the real binary) to
// keep Cobra's own usage/error banner out of the captured output buffers
// used to assert on add's per-line results.
func newTestAddCommand() *cobra.Command {
	cmd := newAddCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
