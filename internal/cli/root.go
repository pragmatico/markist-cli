// Package cli wires up the Cobra command tree: the root command with its
// global flags, and the login/logout/whoami/add/search/list/shared/docs/
// version/man subcommands, plus Cobra's built-in completion command.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/api"
	"github.com/pragmatico/markist-cli/internal/update"
)

// globalFlags holds the root command's persistent flags, shared by every
// subcommand.
type globalFlags struct {
	json       bool
	plain      bool
	noColor    bool
	debug      bool
	apiURL     string
	hyperlinks bool
}

var flags globalFlags

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "markist",
		Short: "Markist — a bookmark and prompt manager, from your terminal",
		Long: `markist is the command-line client for Markist (https://markist.xyz).

Save links, search your library, and browse bookmarks shared with you --
without leaving the terminal.

Getting started:
  1. markist login   -- sign in via your browser (OAuth device flow)
  2. markist add     -- save a URL to your library
  3. markist search  -- find it again later

Run "markist docs" for the full guide, or "markist <command> --help" for a
command's flags and examples.`,
		Example: `  markist login
  markist add https://example.com --read-later
  markist search "rust async runtimes" --tag rust`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			updateResultCh = startUpdateCheck(cmd.Context())
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&flags.json, "json", false, "output machine-readable JSON")
	root.PersistentFlags().BoolVar(&flags.plain, "plain", false, "output plain text instead of the interactive picker")
	root.PersistentFlags().BoolVar(&flags.noColor, "no-color", false, "disable colored output (also honors NO_COLOR)")
	root.PersistentFlags().BoolVar(&flags.debug, "debug", false, "log requests to stderr, with tokens redacted")
	root.PersistentFlags().StringVar(&flags.apiURL, "api-url", "", "override the Markist API base URL")
	_ = root.PersistentFlags().MarkHidden("api-url")
	root.PersistentFlags().BoolVar(&flags.hyperlinks, "hyperlinks", false, "emit OSC 8 hyperlinks in --plain output even when stdout isn't a terminal")

	root.AddCommand(
		newLoginCommand(),
		newLogoutCommand(),
		newWhoamiCommand(),
		newAddCommand(),
		newSearchCommand(),
		newListCommand(),
		newSharedCommand(),
		newDocsCommand(),
		newVersionCommand(),
		newManCommand(),
	)

	return root
}

// updateResultCh is set by the root command's PersistentPreRunE, once flags
// are parsed, and read back here once the command has finished. It's a
// package var (not a local in Execute) because PersistentPreRunE has no
// other way to hand its channel back to the caller of ExecuteContext.
var updateResultCh <-chan *update.Result

// Execute runs the root command and returns a process exit code (see
// exitcode.go). cmd/markist/main.go calls this directly. The context is
// canceled on SIGINT, so a long-running command like login's device-flow
// poll can stop cleanly on Ctrl-C instead of being killed mid-write.
//
// A daily GitHub release check (internal/update) starts in the background
// as soon as flags are parsed, running concurrently with the command itself
// so it never delays the command's own output; once the command has
// finished and printed everything it's going to print, Execute waits for
// that check (bounded by update.CheckTimeout) and prints any available-
// update notice to stderr.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	updateResultCh = nil
	root := newRootCommand()

	err := root.ExecuteContext(ctx)
	code := ExitSuccess
	if err != nil {
		fmt.Fprintln(os.Stderr, ErrorPrefix+err.Error())
		code = exitCodeForError(err)
	}

	if updateResultCh != nil {
		printUpdateNotice(os.Stderr, <-updateResultCh)
	}

	return code
}

// ErrorPrefix is prepended to any error printed by Execute.
const ErrorPrefix = "Error: "

// exitCodeForError maps a command error to a process exit code: a revoked
// or expired session (api.ErrSessionExpired, from the client's refresh
// logic) or no session at all (ErrNotLoggedIn) exits 3, a server-mandated
// version bump (upgrade_required) exits 4, everything else is the generic
// error exit.
func exitCodeForError(err error) int {
	if errors.Is(err, api.ErrSessionExpired) || errors.Is(err, ErrNotLoggedIn) {
		return ExitNotLoggedIn
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.Code == api.ErrorCodeUpgradeRequired {
		return ExitUpgradeRequired
	}
	return ExitError
}
