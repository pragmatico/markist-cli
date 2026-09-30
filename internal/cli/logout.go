package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/config"
)

func newLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current session and remove saved credentials",
		Long: `Revokes the current API session on the server (best-effort -- an offline
revoke failure is reported, not fatal) and removes the saved tokens from
~/.markist/config.`,
		Example: `  markist logout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(cmd)
		},
	}
	return cmd
}

func runLogout(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" && cfg.Auth.RefreshToken == "" {
		fmt.Fprintln(out, "Not logged in.")
		return nil
	}

	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)
	client := api.NewClientFromConfig(apiURL, cfg)

	if err := client.Revoke(cmd.Context()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: couldn't revoke the session on the server (%v). Removing local credentials anyway.\n", err)
	}

	cfg.Auth = config.Auth{}
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Fprintln(out, "Logged out.")
	return nil
}
