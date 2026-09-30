# markist-cli

The command-line client for [Markist](https://markist.xyz) -- a bookmark and
prompt manager. Save links, search your library, and browse bookmarks shared
with you without leaving the terminal.

The binary is `markist`.

## Install

**macOS / Linux**

```bash
curl -fsSL https://markist.xyz/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://markist.xyz/install.ps1 | iex
```

Both scripts install the latest release. They download the archive plus
`checksums.txt` and check the sha256 before installing; if it doesn't match
they stop without installing anything. `install.sh` installs to
`/usr/local/bin` if it's writable, otherwise `~/.local/bin`. `install.ps1`
installs to `%LOCALAPPDATA%\Programs\markist` and adds that folder to your
user `PATH`. To install a specific version, set `MARKIST_VERSION` (for
example `MARKIST_VERSION=0.1.0`).

**Homebrew**

```bash
brew install pragmatico/tap/markist-cli
```

**Go** (Go 1.25+)

```bash
go install github.com/pragmatico/markist-cli/cmd/markist@latest
```

Prebuilt archives for linux, darwin and windows (amd64 and arm64) are also
attached to every [GitHub Release](https://github.com/pragmatico/markist-cli/releases).
Each archive includes man pages and shell completions.

## Getting started

```bash
markist login                         # sign in through your browser
markist add https://example.com       # save a link
markist search "rust async runtimes"  # find it again later
```

`markist login` uses the OAuth device flow. It prints a short code and opens
`https://markist.xyz/device`, where you confirm that the code matches and
approve the request. If you don't have a Markist account yet, create one at
[markist.xyz](https://markist.xyz) first.

## Commands

| Command | What it does |
|---------|--------------|
| `markist login` | Sign in through your browser. `--no-browser` prints the URL instead of opening it; `--force` signs in again. |
| `markist logout` | Revoke the current session and remove the saved credentials. |
| `markist whoami` | Show the signed-in user. |
| `markist add [url]` | Save a URL to your library. With no URL, it reads one URL per line from stdin (`pbpaste \| markist add`). Flags: `--read-later`, `--tag <name>` (repeatable), `--open`. |
| `markist search <query...>` | Semantic + keyword search over your library. Flags: `--tag <name>` (repeatable, all must match), `--read-later`, `--limit N` (default 50, max 200), `--open N` (open the Nth result directly). |
| `markist list` | Browse your library without a query. Flags: `--tag`, `--read-later`, `--limit`. |
| `markist shared` | Bookmarks other Markist users have shared with you. `--notes` shows the shared notes inline. |
| `markist docs` | Open the [CLI guide](https://markist.xyz/guide#cli) in your browser. |
| `markist version` | Print the version, commit, build date and platform. |
| `markist completion bash\|zsh\|fish\|powershell` | Print a shell completion script. |

Title, summary and tags are generated in the background after `add`, and
show up a few seconds later. Run `markist <command> --help` to see every
flag and example for a command.

### Global flags

| Flag | Effect |
|------|--------|
| `--json` | Machine-readable JSON output. |
| `--plain` | Plain text instead of the interactive picker. This is automatic when stdout isn't a terminal. |
| `--no-color` | Disable colors. `NO_COLOR` is also honored. |
| `--hyperlinks` | Emit clickable (OSC 8) links in `--plain` output even when stdout isn't a terminal. |
| `--debug` | Log requests to stderr, with tokens redacted. |

### Interactive picker

In a terminal, `search`, `list` and `shared` open an interactive list:

| Key | Action |
|-----|--------|
| `↑`/`↓`, `j`/`k` | Move |
| `/` | Filter |
| `enter` | Open the link and quit |
| `o` | Open the link and keep the list open |
| `c` | Copy the URL |
| `s` | (`shared` only) Open the Markist page for the share instead of the original link |
| `?` | Toggle help |
| `q`, `esc` | Quit |

## Configuration

Your session is stored in `~/.markist/config`
(`%USERPROFILE%\.markist\config` on Windows). The file is readable only by
you (mode 0600). `markist logout` removes the session and revokes it on the
server. You can also see and revoke CLI sessions under **Connected apps** on
your [Markist account page](https://markist.xyz/account).

| Environment variable | Effect |
|----------------------|--------|
| `MARKIST_CONFIG_DIR` | Use a different directory instead of `~/.markist`. |
| `MARKIST_NO_UPDATE_CHECK` | Turn off the daily "a new version is available" check. The check also doesn't run under `--json`, with `CI` set, or when stdout isn't a terminal. |
| `NO_COLOR` | Disable colored output. |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error |
| `3` | Not logged in, or the session expired -- run `markist login` |
| `4` | This version is too old for the server -- upgrade `markist` |

## Building from source

```bash
go build ./...
go vet ./...
go test ./... -race
golangci-lint run   # uses .golangci.yml (golangci-lint v1.64.8)
go run ./cmd/markist --help
```

Builds made with `go build` report their version as `dev`. The release
pipeline sets the real version at build time.

## Releasing

Releases are cut from this repository with a bare semver tag on `main`. The
tag is also the Go module version that `go install ...@vX.Y.Z` resolves.

```bash
git checkout main && git pull
git tag vX.Y.Z
git push origin vX.Y.Z
```

The tag push triggers `.github/workflows/cli-release.yml`. That workflow runs
GoReleaser (`.goreleaser.yaml`), which:

- cross-compiles linux/darwin/windows for amd64 and arm64
- generates man pages and shell completions and bundles them into the archives
- publishes a GitHub Release with the archives and `checksums.txt`
- updates the Homebrew cask in `pragmatico/homebrew-tap`

The install scripts, `brew` and `markist`'s own update check all use the
latest release, so they pick up a new version as soon as it's published.

GitHub runs the workflow file from the tagged commit. A tag on a commit that
doesn't contain `.github/workflows/cli-release.yml` triggers nothing.

The workflow needs a `HOMEBREW_TAP_TOKEN` repository secret: a fine-grained
PAT with `contents: write` on `pragmatico/homebrew-tap` only.
