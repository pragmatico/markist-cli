// Package update implements the daily release-check notice (plan Task 14):
// at most once per 24h, check GitHub for the newest vX.Y.Z release and print an
// upgrade hint to stderr after the command's own output.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CheckInterval is the minimum time between checks.
const CheckInterval = 24 * time.Hour

// CheckTimeout bounds how long Check waits for GitHub before giving up, so
// a slow or unreachable network never delays the command it's piggybacking
// on by more than this.
const CheckTimeout = 2 * time.Second

// releasesPath is GitHub's releases-list endpoint for the public
// pragmatico/markist-cli repo, where the module lives at the root and each
// release is tagged with a bare vX.Y.Z.
const releasesPath = "/repos/pragmatico/markist-cli/releases"

// tagPrefix identifies a CLI release tag among any other tags in the repo.
const tagPrefix = "v"

// defaultAPIBaseURL is the real GitHub API; tests override it with an
// httptest server via Check's apiBaseURL parameter.
const defaultAPIBaseURL = "https://api.github.com"

// HTTPDoer is the minimal HTTP seam Check needs -- *http.Client satisfies
// it, and tests can substitute anything that does.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Result is what a release check reports.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
}

// ShouldCheck reports whether it has been at least CheckInterval since
// lastCheckedAt (the zero time always triggers a check).
func ShouldCheck(lastCheckedAt time.Time) bool {
	return time.Since(lastCheckedAt) >= CheckInterval
}

// Skip reports whether the update check should be skipped entirely: --json
// output, a non-interactive stdout, MARKIST_NO_UPDATE_CHECK=1, or CI set.
func Skip(jsonFlag, interactive bool, getenv func(string) string) bool {
	if jsonFlag || !interactive {
		return true
	}
	if getenv("MARKIST_NO_UPDATE_CHECK") != "" {
		return true
	}
	if getenv("CI") != "" {
		return true
	}
	return false
}

// Check queries the GitHub releases list for the newest vX.Y.Z tag and
// compares it against currentVersion. apiBaseURL empty means the real
// GitHub API; tests point it at an httptest server. The call is bounded by
// CheckTimeout regardless of the context passed in.
func Check(ctx context.Context, client HTTPDoer, apiBaseURL, currentVersion string) (*Result, error) {
	return checkWithTimeout(ctx, client, apiBaseURL, currentVersion, CheckTimeout)
}

// checkWithTimeout is Check with an injectable timeout, so tests can prove
// the timeout actually bounds a hung request without waiting out the real
// CheckTimeout.
func checkWithTimeout(ctx context.Context, client HTTPDoer, apiBaseURL, currentVersion string, timeout time.Duration) (*Result, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+releasesPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github releases: unexpected status %d", resp.StatusCode)
	}

	var releases []release
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("decoding github releases: %w", err)
	}

	result := &Result{CurrentVersion: currentVersion}

	latest, latestParsed, ok := newestCLIVersion(releases)
	if !ok {
		return result, nil
	}
	result.LatestVersion = latest

	current, ok := parseSemver(currentVersion)
	if !ok {
		// An unparsable running version (e.g. "dev", the default for a
		// non-release build) can't be compared -- report the latest
		// release without claiming an update is available.
		return result, nil
	}

	result.HasUpdate = compareSemver(latestParsed, current) > 0
	return result, nil
}

type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// newestCLIVersion picks the highest-semver vX.Y.Z tag among releases,
// ignoring drafts and pre-releases -- neither is something to nudge a user
// towards installing.
func newestCLIVersion(releases []release) (string, [3]int, bool) {
	var best [3]int
	var bestVersion string
	found := false

	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		if !strings.HasPrefix(r.TagName, tagPrefix) {
			continue
		}
		version := strings.TrimPrefix(r.TagName, tagPrefix)
		parsed, ok := parseSemver(version)
		if !ok {
			continue
		}
		if !found || compareSemver(parsed, best) > 0 {
			best = parsed
			bestVersion = version
			found = true
		}
	}

	return bestVersion, best, found
}

// parseSemver parses "X.Y.Z" (an optional leading "v" and any
// "-prerelease"/"+build" suffix are ignored) into comparable major/minor/
// patch integers. It's intentionally lenient -- good enough to order this
// project's own release tags, not a general-purpose semver library.
func parseSemver(version string) ([3]int, bool) {
	var out [3]int

	v := strings.TrimPrefix(version, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}

	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// compareSemver returns -1, 0, or 1 as a is less than, equal to, or greater
// than b.
func compareSemver(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
