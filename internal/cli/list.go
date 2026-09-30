package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/config"
	"github.com/jmbataller/markist/cli/internal/ui"
)

func newListCommand() *cobra.Command {
	var tags []string
	var readLater bool
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Browse your library without a search query",
		Long: `Lists bookmarks from your Markist library, optionally filtered by tag or
scoped to the read-later queue. Same result renderer as search.`,
		Example: `  markist list
  markist list --read-later
  markist list --tag rust --limit 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, tags, readLater, limit)
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tag", nil, "filter by tag (repeatable, AND semantics)")
	cmd.Flags().BoolVar(&readLater, "read-later", false, "list the read-later queue only")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of results (max 200)")

	return cmd
}

func runList(cmd *cobra.Command, tags []string, readLater bool, limit int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" {
		return ErrNotLoggedIn
	}

	// No free text for `list` -- only the #tag filter, same mechanism
	// `search --tag` uses (see buildSearchQuery / CLAUDE.md's #tagname
	// syntax note, the only tag filter the bearer API exposes).
	query := buildSearchQuery(nil, tags)

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

	return renderResults(cmd, bookmarksToItems(resp.Items), renderOptions{
		title:    "Your library",
		emptyMsg: "No bookmarks yet.",
		footer:   ui.Footer(len(resp.Items), resp.Total, resp.Truncated),
	})
}
