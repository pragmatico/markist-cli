package ui

import (
	"encoding/json"
	"fmt"
	"io"
)

// Mode selects how a command renders its results (plan Task 13).
type Mode int

const (
	ModePicker Mode = iota
	ModePlain
	ModeJSON
)

// ResolveMode picks the output mode: --json wins outright, then --plain or
// a non-interactive stdout falls back to plain text, otherwise the
// Bubble Tea picker.
func ResolveMode(jsonFlag, plainFlag bool) Mode {
	switch {
	case jsonFlag:
		return ModeJSON
	case plainFlag || !IsInteractive():
		return ModePlain
	default:
		return ModePicker
	}
}

// WriteJSON writes v to w as indented JSON.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WritePlainLine writes one numbered plain-text result line. Task 13
// replaces this with OSC 8 hyperlinks (TTY/--hyperlinks only) and a dimmed
// hostname.
func WritePlainLine(w io.Writer, index int, title, url string) {
	fmt.Fprintf(w, "%d. %s  %s\n", index, title, url)
}
