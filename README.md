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

Once the Homebrew tap exists (see "Pending manual steps" below):

```bash
brew install pragmatico/tap/markist-cli
```

`go install github.com/pragmatico/markist-cli/cmd/markist@latest` is the
fallback install path for anyone with a Go toolchain -- the `/guide` page
(`src/app/guide/page.tsx`, `id="cli"` section, added in Task 14) already
covers this same command on the web side.

## Release

Tagging `cli/vX.Y.Z` on the main repo triggers
`.github/workflows/cli-release.yml`, which runs GoReleaser (`.goreleaser.yaml`)
from `cli/` to cross-compile linux/darwin/windows (amd64+arm64), generate man
pages and shell completions, and publish a GitHub Release plus a Homebrew
cask update in `pragmatico/homebrew-tap`.

### Tag handling spike -- verified outcome

`go install` of a module living in a repo subdirectory needs the
path-prefixed `cli/vX.Y.Z` tag, but GoReleaser OSS expects a bare semver tag
and has no monorepo support (`monorepo.tag_prefix` is Pro-only -- confirmed
by reading `pkg/config/config.go` in the installed `v2.18.2`: the struct
doesn't exist in this build at all).

The plan's suggested workaround -- pointing `GORELEASER_CURRENT_TAG` /
`GORELEASER_PREVIOUS_TAG` at a stripped version of the prefixed tag -- **does
not hold up**, verified by reading GoReleaser's source rather than just
running `--snapshot` (snapshot mode skips the exact-match/semver validation
that actually breaks, so it can't catch this):

- `internal/pipe/release.CreateRelease` always sets the GitHub Release's
  `TagName` to the literal `ctx.Git.CurrentTag`, and `.Version` (used in
  every archive/checksum/cask name) is just `strings.TrimPrefix(CurrentTag,
  "v")`.
- `internal/pipe/semver.Run` feeds that same raw `ctx.Git.CurrentTag` string
  to `semver.NewVersion` -- a **hard, non-skippable error** unless
  `--skip=validate` is also passed.
- So pointing `GORELEASER_CURRENT_TAG` at the real `cli/v0.1.0` tag (to keep
  the Release attached to it) fails semver parsing outright (a slash isn't
  valid semver) and would corrupt every `.Version`-templated name with a
  literal `/` in it.
- Stripping it to a bare `v0.1.0` string that **isn't actually a git tag**
  parses fine, but then `internal/pipe/git.validate`'s `git describe
  --exact-match --tags --match v0.1.0` fails (`ErrWrongRef`) because no such
  tag exists -- and even bypassed with `--skip=validate`, the GitHub Release
  API auto-creates a *new*, disconnected `v0.1.0` tag against the default
  branch, not the commit that was actually tagged `cli/v0.1.0`.

Neither direction works. **The verified fallback**:
`.github/workflows/cli-release.yml` mirrors the pushed `cli/vX.Y.Z` tag onto
a real, bare `vX.Y.Z` tag on the exact same commit (`git tag vX.Y.Z
cli/vX.Y.Z && git push origin vX.Y.Z`), then drives GoReleaser off *that*
tag via `GORELEASER_CURRENT_TAG`/`GORELEASER_PREVIOUS_TAG`. Every GoReleaser
assumption now holds against something real: exact-match validation passes,
semver parses cleanly, the Release's `TagName` is a tag that actually
exists, and the cask/archive download URLs GoReleaser builds from `.Tag`
resolve correctly. Confirmed empirically too: a local `goreleaser release
--snapshot --clean --skip=publish` run produced correct archives (with man
pages and completions bundled -- verified via `tar -tzf`) and a
`dist/homebrew/Casks/markist-cli.rb` whose URLs are built from the release
tag, e.g. `.../releases/download/v0.0.0/markist-cli_#{version}_....tar.gz`.

**Consequence for `install.sh`/`install.ps1` and anyone browsing releases**:
the CLI's actual GitHub Releases are tagged `vX.Y.Z` (bare), *not*
`cli/vX.Y.Z` -- the `cli/vX.Y.Z` tag exists only so `go install` can resolve
the module and has no GitHub Release attached to it directly. The install
scripts resolve releases by tag shape (`^v[0-9]+\.[0-9]+\.[0-9]+`), so this
is transparent to end users, but worth knowing if you're looking at the tag
list and wondering why there are two tags per release.

This was also fixed along the way: the Task 9 scaffold's `homebrew_casks`
block used the deprecated `binary:` key, which `goreleaser check` now
flags -- changed to the current `binaries: [markist]` list form.

### Pending manual steps (owner-only, in order)

Nothing above can go live until these four one-time steps happen -- none of
them can be done from this environment (no credentials for any of them):

Push the first tag: `git tag cli/v0.1.0 && git push origin cli/v0.1.0`.
   This triggers `.github/workflows/cli-release.yml`, which will itself
   create and push the mirrored bare `v0.1.0` tag described above.



