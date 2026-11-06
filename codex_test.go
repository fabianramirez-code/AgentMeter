package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// realCodexLine is a real (redacted of nothing sensitive, this is usage metadata)
// line shape observed from ~/.codex/sessions rollout files.
const realCodexLine = `{"timestamp":"2026-11-01T05:11:30.679Z","ordinal":150,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000}},"rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":25.0,"window_minutes":300,"resets_at":1793524555},"secondary":{"used_percent":4.0,"window_minutes":10080,"resets_at":1794111355},"credits":{"has_credits":false,"unlimited":false,"balance":"0"},"individual_limit":null,"spend_control_reached":null,"plan_type":"example","rate_limit_reached_type":null}}}`

func TestParseCodexLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    codexRateLimits
		wantErr bool
	}{
		{
			name: "real schema",
			line: realCodexLine,
			want: codexRateLimits{
				PrimaryPct:     25.0,
				PrimaryReset:   time.Unix(1793524555, 0),
				SecondaryPct:   4.0,
				SecondaryReset: time.Unix(1794111355, 0),
			},
		},
		{
			name:    "no rate_limits field",
			line:    `{"timestamp":"x","type":"event_msg","payload":{"type":"other"}}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			line:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCodexLine([]byte(tt.line))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("parseCodexLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLastRateLimitsLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-test.jsonl")
	content := "{\"no\":\"rate limits here\"}\n" +
		`{"payload":{"rate_limits":{"primary":{"used_percent":1.0,"resets_at":100}}}}` + "\n" +
		realCodexLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	line, err := lastRateLimitsLine(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line) != realCodexLine {
		t.Errorf("got last line %q, want the real schema line", line)
	}
}

func TestNewestRolloutFile(t *testing.T) {
	root := t.TempDir()
	oldDay := filepath.Join(root, "2026", "10", "30")
	newDay := filepath.Join(root, "2026", "10", "31")
	if err := os.MkdirAll(oldDay, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDay, 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldDay, "rollout-old.jsonl")
	newFile := filepath.Join(newDay, "rollout-new.jsonl")
	if err := os.WriteFile(oldFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := newestRolloutFile(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != newFile {
		t.Errorf("newestRolloutFile() = %q, want %q", got, newFile)
	}
}

func TestParsePiCodexToken(t *testing.T) {
	tok, err := parsePiCodexToken([]byte(`{"anthropic":{"type":"oauth","access":"a","expires":1},"openai-codex":{"type":"oauth","access":"acc","refresh":"r","expires":1793961696914,"accountId":"acct"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "acc" || tok.AccountID != "acct" || tok.ExpiresAt != 1793961696914 {
		t.Fatalf("got %+v", tok)
	}
	for _, raw := range []string{`{}`, `{"openai-codex":{"type":"api_key","key":"sk"}}`, `{"openai-codex":{"type":"oauth","access":"a"}}`, `nope`} {
		if _, err := parsePiCodexToken([]byte(raw)); err == nil {
			t.Errorf("%s: want error", raw)
		}
	}
}

func TestParseCodexUsage(t *testing.T) {
	body := []byte(`{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":12,"limit_window_seconds":18000,"reset_after_seconds":100,"reset_at":1793952372},"secondary_window":{"used_percent":35,"limit_window_seconds":604800,"reset_after_seconds":200,"reset_at":1794111355}}}`)
	rl, err := parseCodexUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if rl.PrimaryPct != 12 || rl.SecondaryPct != 35 || rl.PrimaryReset.Unix() != 1793952372 || rl.SecondaryReset.Unix() != 1794111355 {
		t.Fatalf("got %+v", rl)
	}
	if _, err := parseCodexUsage([]byte(`{"rate_limit":null}`)); err == nil {
		t.Error("missing rate_limit: want error")
	}
	if _, err := parseCodexUsage([]byte(`nope`)); err == nil {
		t.Error("invalid json: want error")
	}
}

// writeRollout writes a rollout file with realCodexLine (primary 25%, secondary 4%).
func writeRollout(t *testing.T, sessions string, mod time.Time) {
	t.Helper()
	dir := filepath.Join(sessions, "2026", "11", "01")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rollout-2026-11-01T05-00-00-x.jsonl")
	if err := os.WriteFile(p, []byte(realCodexLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(p, mod, mod)
}

func TestRunCodexWithoutPiLogin(t *testing.T) {
	now := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	d := t.TempDir()
	sessions, noAuth := filepath.Join(d, "sessions"), filepath.Join(d, "auth.json")

	// no rollout, nothing cached: same error as before
	_, r := runCodex(now, sessions, []string{noAuth}, "nope", false, time.Hour, providerState{})
	if r.Status != "error" || r.Msg != "no Codex session with rate_limits found" {
		t.Errorf("empty: %+v", r)
	}

	// rollout present: its values are shown
	writeRollout(t, sessions, now.Add(-time.Minute))
	ps, r := runCodex(now, sessions, []string{noAuth}, "nope", false, time.Hour, providerState{})
	if r.Status != "ok" || r.C2Pct != 4 || ps.C2.UsedPct != 4 {
		t.Errorf("rollout: %+v", r)
	}

	// cached data newer than the rollout (from the usage endpoint) wins
	cached := ps
	cached.LastFetch = now.Add(-10 * time.Second)
	cached.C2.UsedPct = 35
	_, r = runCodex(now, sessions, []string{noAuth}, "nope", false, time.Hour, cached)
	if r.C2Pct != 35 {
		t.Errorf("newer cache should win: %+v", r)
	}

	// no rollout any more but something cached: stale, as before
	os.RemoveAll(sessions)
	_, r = runCodex(now, sessions, []string{noAuth}, "nope", false, time.Hour, ps)
	if r.Status != "stale" || r.Msg != "no Codex session, cached values" {
		t.Errorf("stale: %+v", r)
	}
}
