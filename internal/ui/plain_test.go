package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// goldenItems is the fixed set of rows plain_test.go renders: one row with
// every field set, one bare-minimum row (URL-as-title fallback, no
// tags/read-time/label) exercising Description's empty-parts skipping.
func goldenItems() []Item {
	minutes := 7
	return []Item{
		{
			ItemTitle:    "A Tour of Rust Async Runtimes",
			URL:          "https://example.com/articles/rust-async-runtimes",
			Hostname:     "example.com",
			Tags:         []string{"rust", "async"},
			ReadMinutes:  &minutes,
			MatchedLabel: "Matched: tag",
		},
		{
			ItemTitle: "https://example.com/docs/getting-started",
			URL:       "https://example.com/docs/getting-started",
			Hostname:  "example.com",
		},
	}
}

// checkGolden compares got against testdata/name -- this repo has no other
// golden-file convention yet (cli/testdata/api/*.json fixtures are
// hand-authored, not generated), so this is a minimal, self-contained one
// for byte-exact output like OSC 8 escapes. Regenerate with
// `go run ./gengolden` (see internal/ui/gengolden) after a deliberate output
// change; a mismatch otherwise means a regression.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output mismatch for %s:\n--- got ---\n%q\n--- want ---\n%q", name, got, want)
	}
}

func TestWritePlainNoHyperlinks(t *testing.T) {
	var buf bytes.Buffer
	for i, item := range goldenItems() {
		WritePlain(&buf, i+1, item, false)
	}
	checkGolden(t, "plain_no_hyperlinks.golden", buf.Bytes())
}

func TestWritePlainWithHyperlinks(t *testing.T) {
	var buf bytes.Buffer
	for i, item := range goldenItems() {
		WritePlain(&buf, i+1, item, true)
	}
	checkGolden(t, "plain_with_hyperlinks.golden", buf.Bytes())
}

func TestHyperlinkDisabledReturnsLabelUnchanged(t *testing.T) {
	if got := Hyperlink("title", "https://example.com", false); got != "title" {
		t.Fatalf("Hyperlink() = %q, want %q", got, "title")
	}
}

func TestHyperlinkEnabledWrapsInOSC8(t *testing.T) {
	got := Hyperlink("title", "https://example.com", true)
	want := "\x1b]8;;https://example.com\x07title\x1b]8;;\x07"
	if got != want {
		t.Fatalf("Hyperlink() = %q, want %q", got, want)
	}
}

func TestHyperlinksEnabledPrecedence(t *testing.T) {
	// go test's stdout isn't a TTY, so IsInteractive() is false here:
	// HyperlinksEnabled must fall through to the flag.
	if HyperlinksEnabled(false) {
		t.Fatal("HyperlinksEnabled(false) = true in a non-interactive test run")
	}
	if !HyperlinksEnabled(true) {
		t.Fatal("HyperlinksEnabled(true) = false, want true regardless of TTY")
	}
}

func TestFooterTruncated(t *testing.T) {
	got := Footer(50, 312, true)
	want := "Showing 50 of 312. Use --limit to see more."
	if got != want {
		t.Fatalf("Footer() = %q, want %q", got, want)
	}
}

func TestFooterNotTruncatedIsEmpty(t *testing.T) {
	if got := Footer(50, 50, false); got != "" {
		t.Fatalf("Footer() = %q, want empty", got)
	}
}
