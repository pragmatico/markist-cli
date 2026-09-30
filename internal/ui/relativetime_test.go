package ui

import (
	"testing"
	"time"
)

func TestRelativeTimeCompact(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-3 * time.Hour), "3h ago"},
		{"days", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"months", now.Add(-60 * 24 * time.Hour), "2mo ago"},
		{"years", now.Add(-400 * 24 * time.Hour), "1y ago"},
		{"future clamps to just now", now.Add(5 * time.Minute), "just now"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RelativeTimeCompact(c.t, now); got != c.want {
				t.Fatalf("RelativeTimeCompact() = %q, want %q", got, c.want)
			}
		})
	}
}
