package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestOpenLogRotatesWhenTooBig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentmeter.log")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 23, 44, 11, 0, time.Local)
	f, err := openLog(path, 5, now)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	rotated := filepath.Join(filepath.Dir(path), "agentmeter_20261102234411.log")
	if old, _ := os.ReadFile(rotated); string(old) != "0123456789" {
		t.Errorf("rotated log = %q, want the previous log", old)
	}
	if cur, _ := os.ReadFile(path); len(cur) != 0 {
		t.Errorf("new log = %q, want empty", cur)
	}
}

func TestOpenLogAppendsWhenSmall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentmeter.log")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := openLog(path, 1024, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("b\n")
	f.Close()

	if cur, _ := os.ReadFile(path); string(cur) != "a\nb\n" {
		t.Errorf("log = %q, want appended", cur)
	}
}

func TestRunClaudeTokenSelection(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	d := t.TempDir()
	cp, pp := filepath.Join(d, "c.json"), filepath.Join(d, "a.json")
	run := func(cc, pi string) result {
		os.Remove(cp)
		os.Remove(pp)
		if cc != "" {
			os.WriteFile(cp, []byte(cc), 0o600)
		}
		if pi != "" {
			os.WriteFile(pp, []byte(pi), 0o600)
		}
		_, r := runClaude(now, cp, pp, "nope", filepath.Join(d, "p"), filepath.Join(d, "s"), time.Hour, providerState{})
		return r
	}
	if r := run(`{"claudeAiOauth":{"accessToken":"SECRET1","expiresAt":10}}`, `{"anthropic":{"type":"oauth","access":"SECRET2","expires":10}}`); r.Status != "stale" || r.Msg != "login expired, open claude or gentle-shell" || strings.Contains(r.C1Reset, "SECRET") {
		t.Errorf("both expired: %+v", r)
	}
	if r := run("", ""); r.Status != "error" || r.Msg != "not logged into Claude" {
		t.Errorf("none: %+v", r)
	}
	if r := run("", `{"anthropic":{"type":"api_key","key":"SECRET3"}}`); r.Msg != "not logged into Claude" {
		t.Errorf("api_key only: %+v", r)
	}
	// pre-existing behavior: unreadable Claude Code credentials keep their message
	if r := run(`not json`, ""); r.Status != "error" || r.Msg != "could not read Claude credentials" {
		t.Errorf("malformed claude creds: %+v", r)
	}
	pi := `{"anthropic":{"type":"oauth","access":"x","expires":10}}`
	run("", pi)
	if after, _ := os.ReadFile(pp); string(after) != pi {
		t.Error("auth.json mutated")
	}
}
