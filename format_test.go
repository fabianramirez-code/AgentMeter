package main

import (
	"testing"
	"time"
)

func TestFormatReset(t *testing.T) {
	now := time.Date(2026, 11, 1, 2, 21, 0, 0, time.UTC)

	tests := []struct {
		name    string
		resetAt time.Time
		want    string
	}{
		{
			name:    "within a day",
			resetAt: time.Date(2026, 11, 1, 16, 35, 0, 0, time.UTC), // 14h14m away
			want:    "resets 4:35pm in 14h 14m",
		},
		{
			name:    "several days out, round hour",
			resetAt: time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC), // Thu, 4d 6h55m away
			want:    "resets Thu 9am in 4d 6h",
		},
		{
			name:    "already passed",
			resetAt: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
			want:    "",
		},
		{
			name:    "exactly now",
			resetAt: now,
			want:    "",
		},
		{
			name:    "zero value",
			resetAt: time.Time{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatReset(now, tt.resetAt)
			if got != tt.want {
				t.Errorf("formatReset() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveUsedPct(t *testing.T) {
	now := time.Date(2026, 11, 1, 2, 21, 0, 0, time.UTC)

	tests := []struct {
		name      string
		resetAt   time.Time
		cachedPct int
		want      int
	}{
		{"future reset keeps cached", now.Add(time.Hour), 42, 42},
		{"past reset zeroes", now.Add(-time.Hour), 42, 0},
		{"exact now zeroes", now, 42, 0},
		{"zero resetAt keeps cached", time.Time{}, 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveUsedPct(now, tt.resetAt, tt.cachedPct)
			if got != tt.want {
				t.Errorf("effectiveUsedPct() = %d, want %d", got, tt.want)
			}
		})
	}
}
