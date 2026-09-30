// Package version holds build-time metadata injected via -ldflags by
// GoReleaser (cli/.goreleaser.yaml). Left at their defaults for `go build`
// and `go run` during local development.
package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
