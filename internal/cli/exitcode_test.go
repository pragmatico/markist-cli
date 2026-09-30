package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pragmatico/markist-cli/internal/api"
)

func TestExitCodeForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"session expired", api.ErrSessionExpired, ExitNotLoggedIn},
		{"wrapped session expired", fmt.Errorf("login: %w", api.ErrSessionExpired), ExitNotLoggedIn},
		{"not logged in", ErrNotLoggedIn, ExitNotLoggedIn},
		{"upgrade required", &api.Error{StatusCode: 426, Code: api.ErrorCodeUpgradeRequired}, ExitUpgradeRequired},
		{"other api error", &api.Error{StatusCode: 500, Code: api.ErrorCodeInternal}, ExitError},
		{"generic error", errors.New("boom"), ExitError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeForError(tc.err); got != tc.want {
				t.Fatalf("exitCodeForError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
