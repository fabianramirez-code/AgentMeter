package main

import (
	"testing"
	"time"
)

func TestCardsToResultOmitsC3WhenEmpty(t *testing.T) {
	now := time.Now()
	ps := providerState{
		C1: cardState{Label: "SESSION (5H)", UsedPct: 10, ResetAt: now.Add(time.Hour)},
		C2: cardState{Label: "WEEKLY (7D)", UsedPct: 20, ResetAt: now.Add(time.Hour)},
	}

	r := cardsToResult("ok", "", now, ps)

	if r.C3Label != "" || r.C3Pct != 0 || r.C3Reset != "" {
		t.Errorf("expected empty C3 fields, got Label=%q Pct=%d Reset=%q", r.C3Label, r.C3Pct, r.C3Reset)
	}
}

func TestCardsToResultIncludesC3WhenPresent(t *testing.T) {
	now := time.Now()
	resetAt := now.Add(2 * time.Hour)
	ps := providerState{
		C1: cardState{Label: "SESSION (5H)", UsedPct: 10, ResetAt: now.Add(time.Hour)},
		C2: cardState{Label: "WEEKLY (7D)", UsedPct: 20, ResetAt: now.Add(time.Hour)},
		C3: cardState{Label: "WEEKLY FABLE (7D)", UsedPct: 22, ResetAt: resetAt},
	}

	r := cardsToResult("ok", "", now, ps)

	if r.C3Label != "WEEKLY FABLE (7D)" {
		t.Errorf("C3Label = %q, want %q", r.C3Label, "WEEKLY FABLE (7D)")
	}
	if r.C3Pct != 22 {
		t.Errorf("C3Pct = %d, want 22", r.C3Pct)
	}
	if r.C3Reset == "" {
		t.Error("expected non-empty C3Reset")
	}
}
