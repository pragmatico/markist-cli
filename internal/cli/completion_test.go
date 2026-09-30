package cli

import (
	"strings"
	"testing"
)

// TestCompletionIsWiredForEveryShell guards plan Task 14's "don't just
// assume Cobra adds it for free" instruction: this actually invokes
// `markist completion <shell>` through the real command tree and checks
// for shell-specific markers in the generated script, rather than assuming
// the subcommand exists.
func TestCompletionIsWiredForEveryShell(t *testing.T) {
	withConfigDir(t)
	t.Setenv("MARKIST_NO_UPDATE_CHECK", "1")

	tests := []struct {
		shell  string
		marker string
	}{
		{"bash", "bash completion V2 for markist"},
		{"zsh", "#compdef markist"},
		{"fish", "fish completion for markist"},
		{"powershell", "powershell completion for markist"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			root := newRootCommand()
			var out strings.Builder
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{"completion", tt.shell})

			if err := root.Execute(); err != nil {
				t.Fatalf("markist completion %s error: %v", tt.shell, err)
			}

			got := out.String()
			if !strings.Contains(got, tt.marker) {
				t.Errorf("completion %s output missing %q; got %d bytes", tt.shell, tt.marker, len(got))
			}
			if len(got) < 100 {
				t.Errorf("completion %s output suspiciously short (%d bytes): %q", tt.shell, len(got), got)
			}
		})
	}
}
