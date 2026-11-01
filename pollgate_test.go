package main

import (
	"testing"
	"time"
)

func TestShouldFetch(t *testing.T) {
	now := time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)
	fallback := 20 * time.Minute
	minInterval := 5 * time.Minute
	never := time.Time{}

	tests := []struct {
		name            string
		lastAttempt     time.Time
		lastFetch       time.Time
		processRunning  bool
		activityChanged bool
		want            bool
	}{
		{"never fetched", never, never, false, false, true},
		{"stale beyond fallback", now.Add(-21 * time.Minute), now.Add(-21 * time.Minute), false, false, true},
		{"fresh, no activity", now.Add(-6 * time.Minute), now.Add(-6 * time.Minute), false, false, false},
		{"fresh, process running but no new activity", now.Add(-6 * time.Minute), now.Add(-6 * time.Minute), true, false, false},
		{"fresh, process running with new activity", now.Add(-6 * time.Minute), now.Add(-6 * time.Minute), true, true, true},
		{"fresh, activity changed but process not running", now.Add(-6 * time.Minute), now.Add(-6 * time.Minute), false, true, false},
		{"exactly at fallback boundary", now.Add(-20 * time.Minute), now.Add(-20 * time.Minute), false, false, true},
		{"activity but attempted within min interval", now.Add(-1 * time.Minute), now.Add(-1 * time.Minute), true, true, false},
		{"recent failed attempt, never succeeded", now.Add(-1 * time.Minute), never, false, false, false},
		{"failed attempt past min interval, never succeeded", now.Add(-5 * time.Minute), never, false, false, true},
		{"recent failed attempt blocks fallback retry", now.Add(-2 * time.Minute), now.Add(-30 * time.Minute), false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFetch(now, tt.lastAttempt, tt.lastFetch, minInterval, fallback, tt.processRunning, tt.activityChanged)
			if got != tt.want {
				t.Errorf("shouldFetch() = %v, want %v", got, tt.want)
			}
		})
	}
}
