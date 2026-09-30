package ui

import (
	"fmt"
	"io"
	"time"
)

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is a minimal terminal spinner for a long-running operation like
// login's device-flow poll (plan Task 11). It only animates when stdout is
// interactive (IsInteractive), so piped/CI output and tests never see
// control characters.
type Spinner struct {
	w       io.Writer
	message string
	stop    chan struct{}
	done    chan struct{}
}

// NewSpinner builds a spinner that writes to w with the given message.
func NewSpinner(w io.Writer, message string) *Spinner {
	return &Spinner{w: w, message: message}
}

// Start begins animating in a goroutine. It is a no-op when the terminal
// isn't interactive.
func (s *Spinner) Start() {
	if !IsInteractive() {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	go func() {
		defer close(s.done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-s.stop:
				fmt.Fprint(s.w, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(s.w, "\r%s %s", spinnerFrames[frame%len(spinnerFrames)], s.message)
				frame++
			}
		}
	}()
}

// Stop halts the animation and clears the line, blocking until the
// goroutine started by Start has exited. Safe to call even if Start was a
// no-op (non-interactive).
func (s *Spinner) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
}
