package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pragmatico/markist-cli/internal/config"
	"github.com/pragmatico/markist-cli/internal/ui"
	"github.com/pragmatico/markist-cli/internal/update"
	"github.com/pragmatico/markist-cli/internal/version"
)

// updateHTTPClient, updateAPIBaseURL and updateIsInteractive are test seams
// over the GitHub API call and the TTY check -- production leaves them at
// their real defaults.
var (
	updateHTTPClient    update.HTTPDoer = http.DefaultClient
	updateAPIBaseURL                    = ""
	updateIsInteractive                 = ui.IsInteractive
)

// releasesURL is where printUpdateNotice points the user for the actual
// download: the public repo's GitHub Releases, which also back the install
// scripts and the Homebrew cask.
const releasesURL = "https://github.com/pragmatico/markist-cli/releases"

// shouldRunUpdateCheck applies Task 14's skip/cadence rules: --json, a
// non-interactive stdout, MARKIST_NO_UPDATE_CHECK, CI, or a check within the
// last 24h all skip a run.
func shouldRunUpdateCheck(cfg *config.Config, jsonFlag, interactive bool) bool {
	if update.Skip(jsonFlag, interactive, os.Getenv) {
		return false
	}
	lastChecked, _ := time.Parse(time.RFC3339, cfg.Update.LastCheckedAt)
	return update.ShouldCheck(lastChecked)
}

// startUpdateCheck kicks off the GitHub release check in the background, if
// it's due, and returns a channel that always receives exactly one value --
// nil when the check was skipped, failed, or found no update. The network
// call runs in a goroutine bounded by update.CheckTimeout, so it never
// delays the command that triggered it; Execute reads the channel only
// after the command has already produced its own output.
func startUpdateCheck(ctx context.Context) <-chan *update.Result {
	ch := make(chan *update.Result, 1)

	cfg, err := config.Load()
	if err != nil || !shouldRunUpdateCheck(cfg, flags.json, updateIsInteractive()) {
		ch <- nil
		return ch
	}

	// Record the attempt now, synchronously, so a slow or failing check
	// doesn't retry on every invocation within the same 24h window.
	cfg.Update.LastCheckedAt = time.Now().Format(time.RFC3339)
	_ = config.Save(cfg)

	go func() {
		result, err := update.Check(ctx, updateHTTPClient, updateAPIBaseURL, version.Version)
		if err != nil || result == nil || !result.HasUpdate {
			ch <- nil
			return
		}

		if saved, lerr := config.Load(); lerr == nil {
			saved.Update.LatestSeen = result.LatestVersion
			_ = config.Save(saved)
		}
		ch <- result
	}()

	return ch
}

// printUpdateNotice writes an available-update line to w (stderr in
// production) after the command's own output. A nil result, or one with no
// update, prints nothing.
func printUpdateNotice(w io.Writer, result *update.Result) {
	if result == nil || !result.HasUpdate {
		return
	}
	fmt.Fprintf(w, "\nmarkist %s is available (you have %s). See %s\n",
		result.LatestVersion, result.CurrentVersion, releasesURL)
}
