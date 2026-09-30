package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/config"
	"github.com/jmbataller/markist/cli/internal/ui"
)

func newSearchCommand() *cobra.Command {
	var tags []string
	var readLater bool
	var limit int
	var open int

	cmd := &cobra.Command{
		Use:   "search [query...]",
		Short: "Search your library",
		Long: `Searches your Markist library by free text, tags, or both. Results render
as an interactive picker in a terminal, or a numbered plain-text list when
piped or run with --plain.`,
		Example: `  markist search rust async runtimes
  markist search --tag rust --tag cli
  markist search "vector databases" --limit 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args, tags, readLater, limit, open)
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tag", nil, "filter by tag (repeatable, AND semantics)")
	cmd.Flags().BoolVar(&readLater, "read-later", false, "search the read-later queue only")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of results (max 200)")
	cmd.Flags().IntVar(&open, "open", 0, "open the Nth result without showing the picker")

	return cmd
}

func runSearch(cmd *cobra.Command, args []string, tags []string, readLater bool, limit, open int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" {
		return ErrNotLoggedIn
	}

	query := buildSearchQuery(args, tags)

	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)
	client := api.NewClientFromConfig(apiURL, cfg)

	resp, err := client.ListBookmarks(cmd.Context(), api.ListBookmarksParams{
		Query:     query,
		ReadLater: readLater,
		Limit:     clampLimit(limit),
	})
	if err != nil {
		return err
	}

	if flags.json {
		return ui.WriteJSON(cmd.OutOrStdout(), resp)
	}

	emptyMsg := "No bookmarks match your search."
	if query != "" {
		emptyMsg = fmt.Sprintf("No bookmarks match %q.", query)
	}

	return renderResults(cmd, bookmarksToItems(resp.Items), renderOptions{
		open:     open,
		title:    "Search results",
		emptyMsg: emptyMsg,
		footer:   ui.Footer(len(resp.Items), resp.Total, resp.Truncated),
	})
}
