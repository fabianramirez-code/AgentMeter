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
