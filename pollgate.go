package main

import "time"

// minFetchInterval is the minimum gap between two network attempts for the
// same provider, successful or not. Without it, an active CLI (always running,
// logs always changing) would trigger a request on every skin run/refresh and
// the undocumented endpoints answer 429 Too Many Requests.
const minFetchInterval = 5 * time.Minute

// shouldFetch decides whether a network provider should be polled this run.
// lastAttempt is the last request made (even if it failed); lastFetch is the
// last successful one. See docs/design.md "Poll gating".
func shouldFetch(now, lastAttempt, lastFetch time.Time, minInterval, fallback time.Duration, processRunning, activityChanged bool) bool {
	if now.Sub(lastAttempt) < minInterval {
		return false
	}
	if lastFetch.IsZero() || now.Sub(lastFetch) >= fallback {
		return true
	}
	return processRunning && activityChanged
}
