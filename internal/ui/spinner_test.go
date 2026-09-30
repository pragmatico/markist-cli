package ui

import (
	"bytes"
	"testing"
)

func TestSpinnerStartStopNoopWhenNotInteractive(t *testing.T) {
	// go test's stdout isn't a TTY, so IsInteractive() is false here and
	// Start must not launch a goroutine that Stop then has to join.
	var buf bytes.Buffer
	s := NewSpinner(&buf, "waiting")
	s.Start()
	s.Stop()

	if buf.Len() != 0 {
		t.Fatalf("buf = %q, want no output when non-interactive", buf.String())
	}
}

func TestSpinnerStopWithoutStartIsSafe(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "waiting")
	s.Stop()
}
