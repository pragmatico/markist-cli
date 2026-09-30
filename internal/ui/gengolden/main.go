// Command gengolden bootstraps internal/ui/testdata/*.golden fixtures. Not
// part of the module's normal build; run manually with `go run ./gengolden`
// from internal/ui and delete afterwards.
package main

import (
	"bytes"
	"os"

	"github.com/pragmatico/markist-cli/internal/ui"
)

func main() {
	minutes := 7
	items := []ui.Item{
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

	var noLinks bytes.Buffer
	for i, item := range items {
		ui.WritePlain(&noLinks, i+1, item, false)
	}
	must(os.WriteFile("testdata/plain_no_hyperlinks.golden", noLinks.Bytes(), 0o644))

	var withLinks bytes.Buffer
	for i, item := range items {
		ui.WritePlain(&withLinks, i+1, item, true)
	}
	must(os.WriteFile("testdata/plain_with_hyperlinks.golden", withLinks.Bytes(), 0o644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
