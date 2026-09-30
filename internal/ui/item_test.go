package ui

import "testing"

func TestItemTitleAndFilterValue(t *testing.T) {
	i := Item{ItemTitle: "Rust Async Runtimes", Hostname: "example.com", Tags: []string{"rust", "async"}}

	if got := i.Title(); got != "Rust Async Runtimes" {
		t.Fatalf("Title() = %q", got)
	}
	if got := i.FilterValue(); got != "Rust Async Runtimes example.com rust async" {
		t.Fatalf("FilterValue() = %q", got)
	}
}

func TestItemDescriptionComposesAllSetFields(t *testing.T) {
	minutes := 7
	i := Item{
		Hostname:     "example.com",
		Tags:         []string{"rust", "async"},
		ReadMinutes:  &minutes,
		MatchedLabel: "Matched: tag",
	}

	want := "example.com · #rust #async · ~7 min · Matched: tag"
	if got := i.Description(); got != want {
		t.Fatalf("Description() = %q, want %q", got, want)
	}
}

func TestItemDescriptionSkipsUnsetFields(t *testing.T) {
	i := Item{Hostname: "example.com"}
	if got := i.Description(); got != "example.com" {
		t.Fatalf("Description() = %q, want %q", got, "example.com")
	}
}

func TestItemDescriptionEmptyWhenNothingSet(t *testing.T) {
	i := Item{}
	if got := i.Description(); got != "" {
		t.Fatalf("Description() = %q, want empty", got)
	}
}

func TestItemDescriptionSharedRowSharerAndNotes(t *testing.T) {
	i := Item{
		Hostname: "example.com",
		Meta:     "@sam_reader · 3d ago",
		Notes:    []string{"Check out the second section"},
	}

	want := "example.com · @sam_reader · 3d ago · notes: Check out the second section"
	if got := i.Description(); got != want {
		t.Fatalf("Description() = %q, want %q", got, want)
	}
}
