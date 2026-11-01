package main

import (
	"fmt"
	"time"
)

// formatReset renders a human-readable countdown to resetAt, e.g.
// "resets 4:35pm in 2h 14m" for resets within 24h, or
// "resets Thu 9am in 4d 7h" for resets further out.
// Returns "" if resetAt is zero or not after now (already reset).
func formatReset(now, resetAt time.Time) string {
	if resetAt.IsZero() || !resetAt.After(now) {
		return ""
	}
	d := resetAt.Sub(now)
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("resets %s in %dh %dm", formatClock(resetAt), h, m)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("resets %s %s in %dd %dh", resetAt.Format("Mon"), formatClock(resetAt), days, hours)
}

// formatClock formats a time as "4:35pm", or "9am" when minutes are zero.
func formatClock(t time.Time) string {
	if t.Minute() == 0 {
		return t.Format("3pm")
	}
	return t.Format("3:04pm")
}

// effectiveUsedPct zeroes out the used percentage once resetAt has passed,
// since we have no fresher data until the next successful fetch.
func effectiveUsedPct(now, resetAt time.Time, cachedPct int) int {
	if !resetAt.IsZero() && !resetAt.After(now) {
		return 0
	}
	return cachedPct
}
