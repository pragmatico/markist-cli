package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/api"
	"github.com/pragmatico/markist-cli/internal/config"
	"github.com/pragmatico/markist-cli/internal/ui"
)

// nowFunc stands in for time.Now so shared's "@sharer · 3d ago" rows are
// deterministic in tests.
var nowFunc = time.Now

func newSharedCommand() *cobra.Command {
	var showNotes bool

	cmd := &cobra.Command{
		Use:   "shared",
		Short: "Browse bookmarks shared with you",
		Long: `Lists bookmarks other Markist users have shared with you. In the picker,
Enter opens the original link and "s" opens the Markist page for that share
(notes, sharer, timestamp).`,
		Example: `  markist shared
  markist shared --notes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShared(cmd, showNotes)
		},
	}

	cmd.Flags().BoolVar(&showNotes, "notes", false, "show shared notes inline")

	return cmd
}

func runShared(cmd *cobra.Command, showNotes bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" {
		return ErrNotLoggedIn
	}

	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)
	client := api.NewClientFromConfig(apiURL, cfg)

	resp, err := client.Shared(cmd.Context())
	if err != nil {
		return err
	}

	if flags.json {
		return ui.WriteJSON(cmd.OutOrStdout(), resp)
	}

	return renderResults(cmd, sharedToItems(resp.Items, nowFunc(), showNotes), renderOptions{
		title:    "Shared with you",
		emptyMsg: "No one has shared anything with you yet.",
		shared:   true,
	})
}
