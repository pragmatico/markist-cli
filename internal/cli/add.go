package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/jmbataller/markist/cli/internal/api"
	"github.com/jmbataller/markist/cli/internal/config"
)

// errAddFailed is returned when at least one URL failed to save, after every
// URL has already been processed and reported -- the per-URL failure lines
// already explain what went wrong, so this only drives the exit code.
var errAddFailed = errors.New("one or more URLs failed to save")

func newAddCommand() *cobra.Command {
	var readLater bool
	var tags []string
	var open bool

	cmd := &cobra.Command{
		Use:   "add [url]",
		Short: "Save a bookmark to your library",
		Long: `Saves a URL to your Markist library. Title, summary and tags are generated
asynchronously and appear within a few seconds.

With no URL argument, reads one URL per line from stdin, so it composes with
other tools (e.g. pbpaste | markist add).`,
		Example: `  markist add https://example.com/article
  markist add https://example.com/article --read-later --tag rust --tag cli
  pbpaste | markist add`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, readLater, tags, open)
		},
	}

	cmd.Flags().BoolVar(&readLater, "read-later", false, "save to the read-later queue instead of the library")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "attach a tag (repeatable)")
	cmd.Flags().BoolVar(&open, "open", false, "open the dashboard after saving")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string, readLater bool, tags []string, open bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Auth.AccessToken == "" {
		return ErrNotLoggedIn
	}

	urls, err := collectAddURLs(cmd, args)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	apiURL := resolveAPIURL(flags.apiURL, cfg.APIURL)
	client := api.NewClientFromConfig(apiURL, cfg)

	anyFailed := false
	anySucceeded := false

	for _, raw := range urls {
		if err := validateAddURL(raw); err != nil {
			anyFailed = true
			if werr := writeAddResult(out, addResult{URL: raw, err: err}); werr != nil {
				return werr
			}
			continue
		}

		resp, err := client.AddBookmark(ctx, api.AddBookmarkRequest{URL: raw, ReadLater: readLater, Tags: tags})
		if err != nil {
			anyFailed = true
			if werr := writeAddResult(out, addResult{URL: raw, err: err}); werr != nil {
				return werr
			}
			continue
		}

		anySucceeded = true
		if werr := writeAddResult(out, addResult{URL: raw, ID: resp.ID}); werr != nil {
			return werr
		}
	}

	if open && anySucceeded {
		dest := dashboardURL(apiURL)
		if err := openBrowser(dest); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Couldn't open your browser automatically. Open: %s\n", dest)
		}
	}

	if anyFailed {
		return errAddFailed
	}
	return nil
}

// collectAddURLs resolves the URLs add should process: a single positional
// argument (unless it's "-"), or one URL per line from stdin. "-" always
// reads stdin explicitly; no argument at all reads stdin only when it's
// piped -- a bare `markist add` on a real terminal would otherwise hang
// waiting for input the user never intended to provide.
func collectAddURLs(cmd *cobra.Command, args []string) ([]string, error) {
	if len(args) == 1 && args[0] != "-" {
		return []string{args[0]}, nil
	}

	in := cmd.InOrStdin()
	if len(args) == 0 {
		if f, ok := in.(*os.File); ok && f == os.Stdin && isatty.IsTerminal(f.Fd()) {
			return nil, errors.New("add requires a URL argument or piped input, e.g. pbpaste | markist add")
		}
	}

	return readLines(in)
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return lines, nil
}

// validateAddURL rejects anything that isn't a parseable http(s) URL before
// it reaches the network -- an obviously bad line shouldn't cost a round
// trip just to be told the same thing by the server.
func validateAddURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid URL %q: must be an absolute http:// or https:// URL", raw)
	}
	return nil
}

// dashboardURL derives the web dashboard URL from the API base URL in use,
// so --open follows a custom --api-url/MARKIST_API_URL just like the API
// calls do, instead of always pointing at production.
func dashboardURL(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return DefaultAPIURL + "/dashboard"
	}
	u.Path = "/dashboard"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// addResult is one line of add's output, success or failure.
type addResult struct {
	URL string
	ID  string
	err error
}

// addJSONLine is addResult's --json shape: {id, url} on success, {url,
// error} on failure. Field order matches the plan.
type addJSONLine struct {
	ID    string `json:"id,omitempty"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

func writeAddResult(out io.Writer, r addResult) error {
	if flags.json {
		line := addJSONLine{ID: r.ID, URL: r.URL}
		if r.err != nil {
			line.Error = r.err.Error()
		}
		data, err := json.Marshal(line)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}

	if r.err != nil {
		_, err := fmt.Fprintf(out, "Failed: %s: %s\n", r.URL, r.err.Error())
		return err
	}
	_, err := fmt.Fprintf(out, "Saved: %s — title and summary will appear in a few seconds\n", r.URL)
	return err
}
