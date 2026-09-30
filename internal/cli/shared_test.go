package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
)

func newTestSharedCommand() *cobra.Command {
	cmd := newSharedCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

func sharedServer(t *testing.T, resp api.SharedListDto) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/shared" {
			t.Errorf("path = %s, want /api/v1/shared", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func sampleSharedItems() []api.SharedItemDto {
	title := "A Great Write-Up"
	sharer := "sam_reader"
	return []api.SharedItemDto{
		{
			ShareID:    "shr_1",
			URL:        "https://example.com/posts/great-write-up",
			Title:      &title,
			Tags:       []string{"reading"},
			SharedBy:   &sharer,
			SharedAt:   "2026-01-15T12:00:00.000Z",
			Notes:      []string{"Check out the second section, it's the best part."},
			MarkistURL: "https://markist.xyz/shared/shr_1",
		},
		{
			ShareID:    "shr_2",
			URL:        "https://example.com/videos/intro",
			SharedBy:   nil,
			SharedAt:   "2026-01-01T00:00:00.000Z",
			Notes:      nil,
			MarkistURL: "https://markist.xyz/shared/shr_2",
		},
	}
}

func TestSharedNotLoggedIn(t *testing.T) {
	withConfigDir(t)

	cmd := newTestSharedCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestSharedEmptyResultMessage(t *testing.T) {
	withConfigDir(t)

	server := sharedServer(t, api.SharedListDto{})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSharedCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("shared RunE error: %v", err)
	}
	want := "No one has shared anything with you yet.\n"
	if out.String() != want {
		t.Fatalf("out = %q, want %q", out.String(), want)
	}
}

func stubNow(t *testing.T, fixed time.Time) {
	t.Helper()
	orig := nowFunc
	nowFunc = func() time.Time { return fixed }
	t.Cleanup(func() { nowFunc = orig })
}

func TestSharedPlainOutputRendersSharerAndRelativeTime(t *testing.T) {
	withConfigDir(t)
	stubNow(t, time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC))

	server := sharedServer(t, api.SharedListDto{Items: sampleSharedItems()})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSharedCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("shared RunE error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "1. A Great Write-Up") {
		t.Fatalf("out missing first title:\n%s", got)
	}
	if !strings.Contains(got, "@sam_reader · 3d ago") {
		t.Fatalf("out missing sharer/relative-time meta:\n%s", got)
	}
	if !strings.Contains(got, "2. https://example.com/videos/intro") {
		t.Fatalf("out missing URL-fallback title for untitled share:\n%s", got)
	}
	if strings.Contains(got, "notes:") {
		t.Fatalf("out includes notes without --notes:\n%s", got)
	}
}

func TestSharedNotesFlagIncludesNoteBodies(t *testing.T) {
	withConfigDir(t)
	stubNow(t, time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC))

	server := sharedServer(t, api.SharedListDto{Items: sampleSharedItems()})
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	cmd := newTestSharedCommand()
	cmd.SetArgs([]string{"--notes"})
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("shared RunE error: %v", err)
	}

	if !strings.Contains(out.String(), "notes: Check out the second section, it's the best part.") {
		t.Fatalf("out missing note body with --notes:\n%s", out.String())
	}
}

// TestSharedJSONOutputMatchesFixtureShape replays
// cli/testdata/api/shared-list.json (validated against SharedListDtoSchema by
// src/lib/api-v1/contract.test.ts on the TS side) verbatim, so --json's
// output must decode to exactly the same struct the server actually returns.
func TestSharedJSONOutputMatchesFixtureShape(t *testing.T) {
	withConfigDir(t)

	fixture, err := os.ReadFile("../../testdata/api/shared-list.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var want api.SharedListDto
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

	cmd := newTestSharedCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("shared RunE error: %v", err)
	}

	var got api.SharedListDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
}

func TestSharedJSONOutputMatchesResponseShape(t *testing.T) {
	withConfigDir(t)

	resp := api.SharedListDto{Items: sampleSharedItems()}
	server := sharedServer(t, resp)
	defer server.Close()
	t.Setenv("MARKIST_API_URL", server.URL)
	saveTestAuth(t, server.URL)

	origJSON := flags.json
	flags.json = true
	t.Cleanup(func() { flags.json = origJSON })

	cmd := newTestSharedCommand()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("shared RunE error: %v", err)
	}

	var got api.SharedListDto
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error: %v", out.String(), err)
	}
	if len(got.Items) != 2 || got.Items[0].ShareID != "shr_1" {
		t.Fatalf("got = %+v", got)
	}
}
