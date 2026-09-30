package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
)

func newTestSearchCommand() *cobra.Command {
	cmd := newSearchCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

func TestSearchNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func bookmarkListServer(t *testing.T, gotQuery *url.Values, resp api.BookmarkListDto) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/bookmarks" {
			t.Errorf("path = %s, want /api/v1/bookmarks", r.URL.Path)
		}
		if gotQuery != nil {
			*gotQuery = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestSearchBuildsQueryFromArgsAndTags(t *testing.T) {
	withConfigDir(t)

	var gotQuery url.Values
	server := bookmarkListServer(t, &gotQuery, api.BookmarkListDto{Items: nil, Total: 0, Truncated: false})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust", "async", "--tag", "cli", "--tag", "tools"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	if got, want := gotQuery.Get("q"), "rust async #cli #tools"; got != want {
		t.Fatalf("q = %q, want %q", got, want)
	}
}

func TestSearchReadLaterAndLimitPassthrough(t *testing.T) {
	withConfigDir(t)

	var gotQuery url.Values
	server := bookmarkListServer(t, &gotQuery, api.BookmarkListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"--read-later", "--limit", "20"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	if gotQuery.Get("readLater") != "true" {
		t.Fatalf("readLater = %q, want true", gotQuery.Get("readLater"))
	}
	if gotQuery.Get("limit") != "20" {
		t.Fatalf("limit = %q, want 20", gotQuery.Get("limit"))
	}
}

func TestSearchLimitClampedTo200(t *testing.T) {
	withConfigDir(t)

	var gotQuery url.Values
	server := bookmarkListServer(t, &gotQuery, api.BookmarkListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"--limit", "10000"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}
	if gotQuery.Get("limit") != "200" {
		t.Fatalf("limit = %q, want 200", gotQuery.Get("limit"))
	}
}

func TestSearchEmptyResultMessage(t *testing.T) {
	withConfigDir(t)

	server := bookmarkListServer(t, nil, api.BookmarkListDto{Items: nil, Total: 0, Truncated: false})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"nonexistent"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	want := `No bookmarks match "nonexistent".` + "\n"
	if out.String() != want {
		t.Fatalf("out = %q, want %q", out.String(), want)
	}
}

func samplePlainBookmarks() []api.BookmarkDto {
	title1 := "A Tour of Rust Async Runtimes"
	minutes := 7
	return []api.BookmarkDto{
		{
			ID:              "bkm_1",
			URL:             "https://example.com/articles/rust-async-runtimes",
			Title:           &title1,
			Tags:            []string{"rust", "async"},
			ReadTimeMinutes: &minutes,
			MatchedVia:      []api.MatchSource{api.MatchSourceSemantic, api.MatchSourceTag},
			CreatedAt:       "2026-01-15T12:00:00.000Z",
		},
		{
			ID:         "bkm_2",
			URL:        "https://example.com/docs/getting-started",
			Tags:       nil,
			MatchedVia: nil,
			CreatedAt:  "2026-02-01T09:30:00.000Z",
		},
	}
}

func TestSearchPlainOutputRendersMatchedLabelAndFooter(t *testing.T) {
	withConfigDir(t)

	server := bookmarkListServer(t, nil, api.BookmarkListDto{
		Items:     samplePlainBookmarks(),
		Total:     312,
		Truncated: true,
	})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "1. A Tour of Rust Async Runtimes") {
		t.Fatalf("out missing first title:\n%s", got)
	}
	if !strings.Contains(got, "Matched: tag") {
		t.Fatalf("out missing tag-priority match label (tag beats semantic):\n%s", got)
	}
	if !strings.Contains(got, "2. https://example.com/docs/getting-started") {
		t.Fatalf("out missing URL-fallback title for untitled bookmark:\n%s", got)
	}
	if !strings.Contains(got, "Showing 2 of 312. Use --limit to see more.") {
		t.Fatalf("out missing truncated footer:\n%s", got)
	}
}

func TestSearchJSONOutputMatchesResponseShape(t *testing.T) {
	withConfigDir(t)

	resp := api.BookmarkListDto{Items: samplePlainBookmarks(), Total: 2, Truncated: false}
	server := bookmarkListServer(t, nil, resp)
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	var got api.BookmarkListDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if got.Total != 2 || len(got.Items) != 2 {
		t.Fatalf("got = %+v", got)
	}
	if got.Items[0].ID != "bkm_1" {
		t.Fatalf("got.Items[0].ID = %q", got.Items[0].ID)
	}
}

func TestSearchOpenNthResultCallsOpenBrowser(t *testing.T) {
	withConfigDir(t)

	server := bookmarkListServer(t, nil, api.BookmarkListDto{Items: samplePlainBookmarks(), Total: 2})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	openedURLs := stubOpenBrowser(t)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust", "--open", "2"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}
	if *openedURLs != "https://example.com/docs/getting-started" {
		t.Fatalf("openBrowser called with %q", *openedURLs)
	}
}

// TestSearchJSONOutputMatchesFixtureShape replays
// cli/testdata/api/bookmark-list.json (validated against BookmarkListDtoSchema
// by src/lib/api-v1/contract.test.ts on the TS side) verbatim from the test
// server, so a decode of --json's output must land on exactly the same
// struct the server actually returns.
func TestSearchJSONOutputMatchesFixtureShape(t *testing.T) {
	withConfigDir(t)

	fixture, err := os.ReadFile("../../testdata/api/bookmark-list.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var want api.BookmarkListDto
	if err := json.Unmarshal(fixture, &want); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("search RunE error: %v", err)
	}

	var got api.BookmarkListDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
}

func TestSearchOpenOutOfRangeIsAnError(t *testing.T) {
	withConfigDir(t)

	server := bookmarkListServer(t, nil, api.BookmarkListDto{Items: samplePlainBookmarks(), Total: 2})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	openedURLs := stubOpenBrowser(t)

	cmd := newTestSearchCommand()
	cmd.SetArgs([]string{"rust", "--open", "5"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, errNoSuchResult) {
		t.Fatalf("err = %v, want errNoSuchResult", err)
	}
	if *openedURLs != "" {
		t.Fatalf("openBrowser called with %q, want not called", *openedURLs)
	}
}
