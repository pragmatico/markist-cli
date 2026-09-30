package cli

import (
	"regexp"
	"strings"
	"testing"
)

func TestVersionPrintsVersionCommitDateAndPlatform(t *testing.T) {
	cmd := newVersionCommand()
	var out strings.Builder
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version RunE error: %v", err)
	}

	got := out.String()
	// Default build (no -ldflags): version.Version/Commit/Date are "dev"/
	// "none"/"unknown" -- GoReleaser overrides them at release build time.
	want := regexp.MustCompile(`^markist dev \(none, unknown\) \w+/\w+\n$`)
	if !want.MatchString(got) {
		t.Errorf("version output = %q, want it to match %s", got, want)
	}
}
