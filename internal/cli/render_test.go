package cli

import (
	"testing"
	"time"

	"github.com/jmbataller/markist/cli/internal/api"
)

func TestBuildSearchQuery(t *testing.T) {
	cases := []struct {
		name string
		args []string
		tags []string
		want string
	}{
		{"text only", []string{"rust", "async"}, nil, "rust async"},
		{"tags only", nil, []string{"rust", "cli"}, "#rust #cli"},
		{"text and tags", []string{"vector databases"}, []string{"rust"}, "vector databases #rust"},
		{"neither", nil, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildSearchQuery(c.args, c.tags); got != c.want {
				t.Fatalf("buildSearchQuery() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDeriveMatchLabelPriority(t *testing.T) {
	cases := []struct {
		name string
		via  []api.MatchSource
		want string
	}{
		{"empty", nil, ""},
		{"tag beats semantic", []api.MatchSource{api.MatchSourceSemantic, api.MatchSourceTag}, "Matched: tag"},
		{"note beats keyword", []api.MatchSource{api.MatchSourceKeyword, api.MatchSourceNote}, "Matched: note"},
		{"keyword beats semantic", []api.MatchSource{api.MatchSourceSemantic, api.MatchSourceKeyword}, "Matched: keyword"},
		{"semantic alone", []api.MatchSource{api.MatchSourceSemantic}, "Matched: meaning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveMatchLabel(c.via); got != c.want {
				t.Fatalf("deriveMatchLabel(%v) = %q, want %q", c.via, got, c.want)
			}
		})
	}
}

func TestHostnameOf(t *testing.T) {
	if got := hostnameOf("https://example.com/foo/bar?x=1"); got != "example.com" {
		t.Fatalf("hostnameOf() = %q", got)
	}
	if got := hostnameOf("not a url"); got != "" {
		t.Fatalf("hostnameOf(invalid) = %q, want empty", got)
	}
}

func TestClampLimit(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 0},
		{-5, 0},
		{50, 50},
		{200, 200},
		{201, 200},
		{100000, 200},
	}
	for _, c := range cases {
		if got := clampLimit(c.in); got != c.want {
			t.Fatalf("clampLimit(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBookmarksToItemsTitleFallsBackToURL(t *testing.T) {
	items := bookmarksToItems([]api.BookmarkDto{
		{URL: "https://example.com/a", Title: nil},
	})
	if items[0].ItemTitle != "https://example.com/a" {
		t.Fatalf("ItemTitle = %q, want URL fallback", items[0].ItemTitle)
	}
}

func TestSharedToItemsSharerFallsBackToSomeoneAndNotesGatedByFlag(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	sharedAt := now.Add(-2 * time.Hour).Format(time.RFC3339)

	items := sharedToItems([]api.SharedItemDto{
		{URL: "https://example.com/a", SharedBy: nil, SharedAt: sharedAt, Notes: []string{"hello"}},
	}, now, false)

	if items[0].Meta != "someone · 2h ago" {
		t.Fatalf("Meta = %q, want %q", items[0].Meta, "someone · 2h ago")
	}
	if items[0].Notes != nil {
		t.Fatalf("Notes = %v, want nil when showNotes is false", items[0].Notes)
	}

	withNotes := sharedToItems([]api.SharedItemDto{
		{URL: "https://example.com/a", SharedAt: sharedAt, Notes: []string{"hello"}},
	}, now, true)
	if len(withNotes[0].Notes) != 1 || withNotes[0].Notes[0] != "hello" {
		t.Fatalf("Notes = %v, want [\"hello\"] when showNotes is true", withNotes[0].Notes)
	}
}

func TestSharedToItemsUnparsableTimestampFallsBackToSharerOnly(t *testing.T) {
	sharer := "sam_reader"
	items := sharedToItems([]api.SharedItemDto{
		{URL: "https://example.com/a", SharedBy: &sharer, SharedAt: "not-a-timestamp"},
	}, time.Now(), false)

	if items[0].Meta != "@sam_reader" {
		t.Fatalf("Meta = %q, want %q", items[0].Meta, "@sam_reader")
	}
}
