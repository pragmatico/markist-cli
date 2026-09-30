package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/api"
	"github.com/pragmatico/markist-cli/internal/ui"
)

// errNoSuchResult is --open's error when N is out of range.
var errNoSuchResult = errors.New("no result at that position")

// maxResultLimit caps what --limit accepts client-side; the server applies
// its own caps (SEMANTIC_MATCH_LIMIT/KEYWORD_MATCH_LIMIT in CLAUDE.md) but a
// client-side ceiling keeps an accidental --limit 100000 from being sent at
// all.
const maxResultLimit = 200

func clampLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit > maxResultLimit {
		return maxResultLimit
	}
	return limit
}

// buildSearchQuery joins free-text args with spaces and appends "#tag" for
// each --tag, per plan Task 13 -- the server's #tagname syntax
// (parseSearchQuery, CLAUDE.md "Search & dashboard") is the only way the
// bearer API takes a tag filter, so list's --tag goes through the same path
// with no free text.
func buildSearchQuery(args []string, tags []string) string {
	parts := make([]string, 0, len(args)+len(tags))
	if text := strings.TrimSpace(strings.Join(args, " ")); text != "" {
		parts = append(parts, text)
	}
	for _, t := range tags {
		parts = append(parts, "#"+t)
	}
	return strings.Join(parts, " ")
}

// matchSourcePriority/matchSourceLabel mirror deriveMatchLabel
// (src/components/dashboard/bookmark-display.tsx): a tag/note match isn't
// visible elsewhere on the row, so it outranks keyword (usually visible
// already) and semantic (no exact text to point to).
var matchSourcePriority = []api.MatchSource{
	api.MatchSourceTag,
	api.MatchSourceNote,
	api.MatchSourceKeyword,
	api.MatchSourceSemantic,
}

var matchSourceLabel = map[api.MatchSource]string{
	api.MatchSourceTag:      "Matched: tag",
	api.MatchSourceNote:     "Matched: note",
	api.MatchSourceKeyword:  "Matched: keyword",
	api.MatchSourceSemantic: "Matched: meaning",
}

func deriveMatchLabel(via []api.MatchSource) string {
	if len(via) == 0 {
		return ""
	}
	set := make(map[api.MatchSource]bool, len(via))
	for _, v := range via {
		set[v] = true
	}
	for _, p := range matchSourcePriority {
		if set[p] {
			return matchSourceLabel[p]
		}
	}
	return ""
}

// hostnameOf returns url's host, or "" if it doesn't parse -- best-effort
// display only, never used for validation.
func hostnameOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// bookmarksToItems converts search/list results into picker/plain rows.
func bookmarksToItems(items []api.BookmarkDto) []ui.Item {
	out := make([]ui.Item, len(items))
	for i, b := range items {
		title := b.URL
		if b.Title != nil && *b.Title != "" {
			title = *b.Title
		}
		out[i] = ui.Item{
			ItemTitle:    title,
			URL:          b.URL,
			Hostname:     hostnameOf(b.URL),
			Tags:         b.Tags,
			ReadMinutes:  b.ReadTimeMinutes,
			MatchedLabel: deriveMatchLabel(b.MatchedVia),
		}
	}
	return out
}

// sharedToItems converts shared items into picker/plain rows. notes are
// only attached when showNotes is set (the --notes flag), matching the
// flag's "show shared notes inline" description.
func sharedToItems(items []api.SharedItemDto, now time.Time, showNotes bool) []ui.Item {
	out := make([]ui.Item, len(items))
	for i, s := range items {
		title := s.URL
		if s.Title != nil && *s.Title != "" {
			title = *s.Title
		}

		by := "someone"
		if s.SharedBy != nil && *s.SharedBy != "" {
			by = "@" + *s.SharedBy
		}
		meta := by
		if sharedAt, err := time.Parse(time.RFC3339, s.SharedAt); err == nil {
			meta = fmt.Sprintf("%s · %s", by, ui.RelativeTimeCompact(sharedAt, now))
		}

		item := ui.Item{
			ItemTitle:    title,
			URL:          s.URL,
			SecondaryURL: s.MarkistURL,
			Hostname:     hostnameOf(s.URL),
			Tags:         s.Tags,
			Meta:         meta,
		}
		if showNotes {
			item.Notes = s.Notes
		}
		out[i] = item
	}
	return out
}

// renderOptions parameterizes renderResults across search/list/shared: the
// same dispatch (open-by-index / plain / picker) with per-command copy and
// picker semantics.
type renderOptions struct {
	open     int // --open N, 1-indexed; 0 means not requested
	title    string
	emptyMsg string
	footer   string
	shared   bool // enables the picker's "s" (open SecondaryURL) binding
}

// renderResults is the shared tail of search/list/shared once results have
// been fetched and converted to ui.Item: --open short-circuits everything
// else, then an empty result set reports emptyMsg, then plain or the
// interactive picker render the rows (json is handled by each command
// before calling this, since its payload shape differs per command).
func renderResults(cmd *cobra.Command, items []ui.Item, opts renderOptions) error {
	if opts.open > 0 {
		if opts.open > len(items) {
			return errNoSuchResult
		}
		return openBrowser(items[opts.open-1].URL)
	}

	out := cmd.OutOrStdout()

	if len(items) == 0 {
		_, err := fmt.Fprintln(out, opts.emptyMsg)
		return err
	}

	if ui.ResolveMode(false, flags.plain) == ui.ModePlain {
		hyperlinks := ui.HyperlinksEnabled(flags.hyperlinks)
		for i, item := range items {
			ui.WritePlain(out, i+1, item, hyperlinks)
		}
		if opts.footer != "" {
			if _, err := fmt.Fprintln(out, opts.footer); err != nil {
				return err
			}
		}
		return nil
	}

	model := ui.NewPickerModel(opts.title, items, opts.shared, openBrowser, ui.CopyToClipboard)
	finalModel, err := tea.NewProgram(model, tea.WithOutput(cmd.OutOrStdout())).Run()
	if err != nil {
		return err
	}
	pm, ok := finalModel.(ui.PickerModel)
	if !ok || pm.Opened == nil {
		return nil
	}
	target := pm.Opened.URL
	if pm.OpenedSecondary {
		target = pm.Opened.SecondaryURL
	}
	return openBrowser(target)
}
