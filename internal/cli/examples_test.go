package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// walkCommands visits cmd and every descendant added before Execute (i.e.
// not cobra's lazily-injected help/completion commands, which aren't part
// of this package's own command tree).
func walkCommands(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, child := range cmd.Commands() {
		walkCommands(child, visit)
	}
}

// TestEveryVisibleCommandHasAnExample guards plan Task 14's documentation
// requirement: every command a user can actually see in --help needs a
// worked example, not just a one-line Short description. A hidden command
// (like `man`, which GoReleaser runs, not a person) is exempt.
func TestEveryVisibleCommandHasAnExample(t *testing.T) {
	root := newRootCommand()

	walkCommands(root, func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		if cmd.Example == "" {
			t.Errorf("command %q has no Example", cmd.CommandPath())
		}
		if cmd.Short == "" {
			t.Errorf("command %q has no Short description", cmd.CommandPath())
		}
		if cmd.Long == "" {
			t.Errorf("command %q has no Long description", cmd.CommandPath())
		}
	})
}

func TestRootHelpHasGettingStartedSection(t *testing.T) {
	root := newRootCommand()

	if got := root.Long; !strings.Contains(got, "Getting started") {
		t.Errorf("root Long = %q, want it to contain a \"Getting started\" section", got)
	}
	for _, cmd := range []string{"markist login", "markist add", "markist search"} {
		if !strings.Contains(root.Long, cmd) {
			t.Errorf("root Long = %q, want it to mention %q", root.Long, cmd)
		}
	}
}
