package main

import (
	"testing"
	"time"
)

// realClaudeUsageBody is a real response body from the undocumented
// /api/oauth/usage endpoint (contains no secrets - it's account usage data,
// trimmed of irrelevant always-null fields for readability).
const realClaudeUsageBody = `{"five_hour":{"utilization":5.0,"resets_at":"2026-11-01T08:20:00.211437+00:00","limit_dollars":null,"used_dollars":null,"remaining_dollars":null,"locked_reason":null},"seven_day":{"utilization":7.0,"resets_at":"2026-11-07T07:00:00.211453+00:00","limit_dollars":null,"used_dollars":null,"remaining_dollars":null,"locked_reason":null},"seven_day_oauth_apps":null}`

func TestParseClaudeUsage(t *testing.T) {
	got, err := parseClaudeUsage([]byte(realClaudeUsageBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFiveHourReset, _ := time.Parse(time.RFC3339Nano, "2026-11-01T08:20:00.211437+00:00")
	wantSevenDayReset, _ := time.Parse(time.RFC3339Nano, "2026-11-07T07:00:00.211453+00:00")

	if got.FiveHour.Utilization != 5.0 {
		t.Errorf("FiveHour.Utilization = %v, want 5.0", got.FiveHour.Utilization)
	}
	if !got.FiveHour.ResetsAt.Equal(wantFiveHourReset) {
		t.Errorf("FiveHour.ResetsAt = %v, want %v", got.FiveHour.ResetsAt, wantFiveHourReset)
	}
	if got.SevenDay.Utilization != 7.0 {
		t.Errorf("SevenDay.Utilization = %v, want 7.0", got.SevenDay.Utilization)
	}
	if !got.SevenDay.ResetsAt.Equal(wantSevenDayReset) {
		t.Errorf("SevenDay.ResetsAt = %v, want %v", got.SevenDay.ResetsAt, wantSevenDayReset)
	}
}

// realClaudeUsageBodyWithFable is a trimmed real response that also carries
// the weekly_scoped per-model ("Fable") limit entry in `limits` (added
// 2026-11-02; there is no top-level seven_day_fable key).
const realClaudeUsageBodyWithFable = `{"five_hour":{"utilization":20.0,"resets_at":"2026-11-03T07:49:59.920264+00:00"},"seven_day":{"utilization":21.0,"resets_at":"2026-11-07T06:59:59.920282+00:00"},"seven_day_opus":null,"seven_day_sonnet":null,"limits":[{"kind":"session","group":"session","percent":20,"severity":"normal","resets_at":"2026-11-03T07:49:59.920264+00:00","scope":null,"is_active":false},{"kind":"weekly_all","group":"weekly","percent":21,"severity":"normal","resets_at":"2026-11-07T06:59:59.920282+00:00","scope":null,"is_active":false},{"kind":"weekly_scoped","group":"weekly","percent":22,"severity":"normal","resets_at":"2026-11-07T06:59:59.920435+00:00","scope":{"model":{"id":null,"display_name":"Fable"},"surface":null},"is_active":true}]}`

func TestScopedWeeklyFound(t *testing.T) {
	u, err := parseClaudeUsage([]byte(realClaudeUsageBodyWithFable))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	window, displayName, ok := scopedWeekly(u)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if displayName != "Fable" {
		t.Errorf("displayName = %q, want %q", displayName, "Fable")
	}
	if window.Utilization != 22.0 {
		t.Errorf("Utilization = %v, want 22.0", window.Utilization)
	}
	wantReset, _ := time.Parse(time.RFC3339Nano, "2026-11-07T06:59:59.920435+00:00")
	if !window.ResetsAt.Equal(wantReset) {
		t.Errorf("ResetsAt = %v, want %v", window.ResetsAt, wantReset)
	}
}

func TestScopedWeeklyAbsent(t *testing.T) {
	u, err := parseClaudeUsage([]byte(realClaudeUsageBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, ok := scopedWeekly(u)
	if ok {
		t.Fatal("expected ok=false when no weekly_scoped limit present")
	}
}

func TestScopedWeeklyNullLimits(t *testing.T) {
	u, err := parseClaudeUsage([]byte(`{"five_hour":{"utilization":1},"seven_day":{"utilization":1},"limits":null}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, ok := scopedWeekly(u)
	if ok {
		t.Fatal("expected ok=false when limits is null")
	}
}

func TestParseClaudeUsageInvalidJSON(t *testing.T) {
	_, err := parseClaudeUsage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIsTokenExpired(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt int64
		want      bool
	}{
		{"expired in the past", now.Add(-time.Hour).UnixMilli(), true},
		{"expires exactly now", now.UnixMilli(), true},
		{"expires in the future", now.Add(time.Hour).UnixMilli(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTokenExpired(now, tt.expiresAt); got != tt.want {
				t.Errorf("isTokenExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}
