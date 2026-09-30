package ui

import (
	"fmt"
	"time"
)

// RelativeTimeCompact renders a compact "Nd ago" label for the shared
// picker's "@sharer · 3d ago" row (plan Task 13) -- deliberately terser than
// the web dashboard's formatRelativeTime (src/lib/format/relative-time.ts),
// which spells out "3 days ago" for a sentence context a one-line row
// doesn't have room for. now is injected so callers/tests are deterministic.
func RelativeTimeCompact(t, now time.Time) string {
	diff := now.Sub(t)
	if diff < 0 {
		diff = 0
	}
	seconds := int(diff.Seconds())

	switch {
	case seconds < 60:
		return "just now"
	case seconds < 3600:
		return fmt.Sprintf("%dm ago", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh ago", seconds/3600)
	case seconds < 30*86400:
		return fmt.Sprintf("%dd ago", seconds/86400)
	case seconds < 365*86400:
		return fmt.Sprintf("%dmo ago", seconds/(30*86400))
	default:
		return fmt.Sprintf("%dy ago", seconds/(365*86400))
	}
}
