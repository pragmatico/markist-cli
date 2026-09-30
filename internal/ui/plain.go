package ui

import (
	"fmt"
	"io"
)

// Hyperlink wraps label in an OSC 8 hyperlink to url when enabled; otherwise
// it returns label unchanged. OSC 8 uses BEL (\x07) as the cheapest widely
// supported terminator.
func Hyperlink(label, url string, enabled bool) string {
	if !enabled {
		return label
	}
	return "\x1b]8;;" + url + "\x07" + label + "\x1b]8;;\x07"
}

// HyperlinksEnabled decides whether plain-mode output should emit OSC 8
// escapes: only when stdout is a real terminal or --hyperlinks was passed
// explicitly -- raw escape codes in a pipe are garbage for whatever
// downstream tool consumes it.
func HyperlinksEnabled(hyperlinksFlag bool) bool {
	return hyperlinksFlag || IsInteractive()
}

// WritePlain renders one numbered plain-text row: the title (as an OSC 8
// hyperlink to item.URL when hyperlinks is true) on its own line, followed
// by a dimmed line with hostname/tags/read time/match label, when any are
// set.
func WritePlain(w io.Writer, index int, item Item, hyperlinks bool) {
	title := Hyperlink(item.Title(), item.URL, hyperlinks)
	fmt.Fprintf(w, "%d. %s\n", index, title)
	if desc := item.Description(); desc != "" {
		fmt.Fprintf(w, "   %s\n", DimStyle.Render(desc))
	}
}

// Footer renders the "Showing N of M. Use --limit to see more." line when
// truncated is true, or "" otherwise.
func Footer(shown, total int, truncated bool) string {
	if !truncated {
		return ""
	}
	return fmt.Sprintf("Showing %d of %d. Use --limit to see more.", shown, total)
}
