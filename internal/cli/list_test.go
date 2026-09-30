package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
)

func newTestListCommand() *cobra.Command {
	cmd := newListCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

func TestListNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newTestListCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestListSendsNoFreeTextQueryByDefault(t *testing.T) {
	withConfigDir(t)

	var gotQuery url.Values
	server := bookmarkListServer(t, &gotQuery, api.BookmarkListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestListCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list RunE error: %v", err)
	}
	if got := gotQuery.Get("q"); got != "" {
		t.Fatalf("q = %q, want empty", got)
	}
}

func TestListTagFilterBuildsHashtagQuery(t *testing.T) {
	withConfigDir(t)

	var gotQuery url.Values
	server := bookmarkListServer(t, &gotQuery, api.BookmarkListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestListCommand()
	cmd.SetArgs([]string{"--tag", "rust", "--read-later", "--limit", "100"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list RunE error: %v", err)
	}
	if got := gotQuery.Get("q"); got != "#rust" {
		t.Fatalf("q = %q, want %q", got, "#rust")
	}
	if gotQuery.Get("readLater") != "true" {
		t.Fatalf("readLater = %q, want true", gotQuery.Get("readLater"))
	}
	if gotQuery.Get("limit") != "100" {
		t.Fatalf("limit = %q, want 100", gotQuery.Get("limit"))
	}
}

func TestListEmptyResultMessage(t *testing.T) {
	withConfigDir(t)

	server := bookmarkListServer(t, nil, api.BookmarkListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestListCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list RunE error: %v", err)
	}
	want := "No bookmarks yet.\n"
	if out.String() != want {
		t.Fatalf("out = %q, want %q", out.String(), want)
	}
}

func TestListJSONOutputMatchesResponseShape(t *testing.T) {
	withConfigDir(t)

	resp := api.BookmarkListDto{Items: samplePlainBookmarks(), Total: 2, Truncated: false}
	server := bookmarkListServer(t, nil, resp)
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestListCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list RunE error: %v", err)
	}

	var got api.BookmarkListDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if got.Total != 2 || len(got.Items) != 2 {
		t.Fatalf("got = %+v", got)
	}
}
