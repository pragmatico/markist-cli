# markist-cli

The command-line client for [Markist](https://markist.xyz) -- a bookmark and
prompt manager. The binary is `markist`; this module is `markist-cli`
(`github.com/pragmatico/markist-cli`).

This README covers building and running the CLI itself.

## Structure

```
cmd/markist/       main() -- delegates entirely to internal/cli.Execute()
internal/cli/       Cobra command tree: root + login/logout/whoami/add/
                    search/list/shared/docs/version/man, global flags
internal/api/       HTTP client, DTO types, error envelope (mirrors
                    src/lib/api-v1/dto.ts and src/lib/api-auth/errors.ts)
internal/auth/      OAuth device-flow types and request/poll functions
internal/config/    ~/.markist/config (TOML) load/save, refresh file lock
internal/ui/        Lip Gloss styles, the Bubble Tea result picker, and the
                    plain/JSON output modes
internal/update/    daily "a new version is available" release check
internal/version/   Version/Commit/Date, set via -ldflags at release time
testdata/api/       fixtures validated against the DTO schemas by
                    src/lib/api-v1/contract.test.ts -- keep both in sync
```

## Commands

```
markist login                          sign in via the browser (OAuth device flow)
markist logout                         revoke the current session, clear local config
markist whoami                         print the signed-in user
markist add <url> [-]                  save a URL (or one per line from stdin) to your library
markist search <query> [--tag t]       semantic + keyword search, interactive picker or --plain
markist list [--tag t] [--read-later]  browse without a query, same picker/renderer as search
markist shared [--notes]               bookmarks shared with you
markist docs                           open the web guide (markist.xyz/guide#cli)
markist version                        build version, commit, date, Go/OS/arch
markist completion bash|zsh|fish|powershell
markist man --dir <path>               generate man pages (hidden; used by the release pipeline)
```

Every command carries `--help` with its own examples (enforced by
`examples_test.go`'s `TestEveryVisibleCommandHasAnExample`). Global flags:
`--json` (machine-readable output), `--plain` (skip the interactive picker),
`--no-color` (also honors `NO_COLOR`), `--debug` (log requests to stderr,
tokens redacted). `search`/`list`/`shared` results open in a Bubble Tea picker
by default (`enter` opens in the browser, `c` copies the URL, `s` on `shared`
opens the Markist page instead of the original link) or render as plain text
under `--plain` / when stdout isn't a terminal.

Session credentials live in `~/.markist/config` (TOML, chmod 0600); a refresh
call takes a `flock` on `~/.markist/config.lock` first, so two `markist`
processes racing an expired access token don't both present the same refresh
token to the server and trip its reuse-replay detection.

## Build & test

```bash
cd cli
go build ./...
go vet ./...
go test ./...
golangci-lint run   # uses .golangci.yml
```

`MARKIST_API_URL` takes precedence over the built-in `https://markist.xyz`
default and below the (hidden) `--api-url` flag. `MARKIST_CONFIG_DIR`
overrides `~/.markist` for isolated test runs.

## Install

```bash
curl -fsSL https://markist.xyz/install.sh | sh          # macOS / Linux
irm https://markist.xyz/install.ps1 | iex                # Windows (PowerShell)
```

Set `MARKIST_VERSION` (e.g. `MARKIST_VERSION=0.1.0`) to pin a version instead
of installing the latest release. Both scripts download the archive plus
`checksums.txt` and verify the sha256 before installing, aborting loudly on a
mismatch. `install.sh` installs to `/usr/local/bin` if writable, else
`~/.local/bin`; `install.ps1` installs to `%LOCALAPPDATA%\Programs\markist`
and adds it to the user `PATH`.

Homebrew:

```bash
brew install pragmatico/tap/markist-cli
```

`go install github.com/pragmatico/markist-cli/cmd/markist@latest` (Go 1.25+)
is the fallback install path for anyone with a Go toolchain -- the
[markist.xyz guide](https://markist.xyz/guide#cli) covers the same command.

## Release

This folder is developed inside the private Markist monorepo and mirrored
to the public [`pragmatico/markist-cli`](https://github.com/pragmatico/markist-cli)
repo on every push to `main` that touches it. The module lives at that
repo's root, so a release is a bare semver tag there:

```bash
git tag v0.1.0 && git push origin v0.1.0   # in pragmatico/markist-cli
```

The tag triggers `.github/workflows/cli-release.yml`, which runs GoReleaser
(`.goreleaser.yaml`) to cross-compile linux/darwin/windows (amd64+arm64),
generate man pages and shell completions, and publish a GitHub Release plus
a Homebrew cask update in `pragmatico/homebrew-tap`. The same `vX.Y.Z` tag is
the Go module version `go install` resolves, and the release `markist`'s
daily update check and the install scripts look for.

The workflow needs a `HOMEBREW_TAP_TOKEN` secret on `pragmatico/markist-cli`:
a fine-grained PAT with `contents: write` on `pragmatico/homebrew-tap` only.
