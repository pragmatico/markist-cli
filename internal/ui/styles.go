// Package ui holds the CLI's terminal presentation: Lip Gloss styles, the
// Bubble Tea result picker, and the plain/JSON output modes (plan Task 13).
// This is the skeleton layer (plan Task 9); real rendering, filtering and
// key handling land in Task 13.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// ColorEnabled applies the --no-color flag and NO_COLOR env var precedence
// from the root command's global flags (plan Task 9).
func ColorEnabled(noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

// IsInteractive reports whether stdout is attached to a real terminal --
// the switch between the Bubble Tea picker and plain/JSON output.
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

var (
	TitleStyle = lipgloss.NewStyle().Bold(true)
	DimStyle   = lipgloss.NewStyle().Faint(true)
	ErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	MutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
