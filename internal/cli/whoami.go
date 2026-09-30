package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/api"
	"github.com/pragmatico/markist-cli/internal/config"
	"github.com/pragmatico/markist-cli/internal/ui"
)

func newWhoamiCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the signed-in user",
		Long:  `Prints the signed-in username and email, plus the API host in use.`,
		Example: `  markist whoami
  markist whoami --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(cmd)
		},
	}
	return cmd
}

func runWhoami(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" {
		return ErrNotLoggedIn
	}

	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)
	client := api.NewClientFromConfig(apiURL, cfg)

	me, err := client.Me(cmd.Context())
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if flags.json {
		return ui.WriteJSON(out, me)
	}

	fmt.Fprintf(out, "@%s (%s)\n", me.Username, me.Email)
	fmt.Fprintln(out, apiURL)
	return nil
}
