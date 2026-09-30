package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/api"
	"github.com/pragmatico/markist-cli/internal/auth"
	"github.com/pragmatico/markist-cli/internal/config"
	"github.com/pragmatico/markist-cli/internal/ui"
)

// openBrowser and confirmAgain are seams over pkg/browser and ui.Confirm:
// both launch real OS/TUI interactions that tests replace with fakes.
var (
	openBrowser  = browser.OpenURL
	confirmAgain = ui.Confirm
)

func newLoginCommand() *cobra.Command {
	var force, noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Markist via your browser",
		Long: `Starts the OAuth device authorization flow: markist prints a short code
and opens https://markist.xyz/device in your browser, where you approve the
request while already signed in. The resulting access and refresh tokens are
saved to ~/.markist/config.`,
		Example: `  markist login
  markist login --force
  markist login --no-browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, force, noBrowser)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "log in again even if already logged in")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the verification URL instead of opening it")

	return cmd
}

func runLogin(cmd *cobra.Command, force, noBrowser bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)

	if !force && cfg.Auth.AccessToken != "" {
		existing := api.NewClientFromConfig(apiURL, cfg)
		if me, meErr := existing.Me(ctx); meErr == nil {
			again, confirmErr := confirmAgain(fmt.Sprintf("Already logged in as @%s. Log in again?", me.Username))
			if confirmErr != nil {
				return confirmErr
			}
			if !again {
				fmt.Fprintf(out, "Staying logged in as @%s.\n", me.Username)
				return nil
			}
		}
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	clientName := fmt.Sprintf("markist-cli on %s", hostname)

	authClient := &auth.Client{BaseURL: apiURL}
	codeResp, err := authClient.RequestDeviceCode(ctx, clientName)
	if err != nil {
		return err
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, ui.DeviceCodeBox(codeResp.UserCode))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Open %s and enter the code above.\n", codeResp.VerificationURI)
	fmt.Fprintln(out)

	if noBrowser {
		fmt.Fprintf(out, "Or open: %s\n", codeResp.VerificationURIComplete)
	} else if err := openBrowser(codeResp.VerificationURIComplete); err != nil {
		fmt.Fprintf(errOut, "Couldn't open your browser automatically. Open: %s\n", codeResp.VerificationURIComplete)
	}

	spin := ui.NewSpinner(errOut, "Waiting for approval…")
	spin.Start()
	tokenResp, err := authClient.PollForToken(
		ctx,
		codeResp.DeviceCode,
		time.Duration(codeResp.Interval)*time.Second,
		time.Duration(codeResp.ExpiresIn)*time.Second,
	)
	spin.Stop()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return errors.New("login canceled")
		}
		return err
	}

	newCfg := *cfg
	newCfg.Auth = config.Auth{
		AccessToken:     tokenResp.AccessToken,
		RefreshToken:    tokenResp.RefreshToken,
		AccessExpiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
		ClientName:      clientName,
	}
	if err := config.Save(&newCfg); err != nil {
		return err
	}

	me, err := api.NewClientFromConfig(apiURL, &newCfg).Me(ctx)
	if err != nil {
		// Tokens are already saved; a transient failure fetching the
		// username right after login shouldn't be reported as a login
		// failure.
		fmt.Fprintln(out, "Logged in.")
		return nil
	}
	fmt.Fprintf(out, "Logged in as @%s\n", me.Username)
	return nil
}
