package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSession(t *testing.T, dir, name, body string, mod time.Time) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestPiActiveProviders(t *testing.T) {
	now := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	d := t.TempDir()
	// recent session that switched from anthropic to openai-codex: last provider wins
	writeSession(t, d, "--E--a--/s1.jsonl", `{"provider":"anthropic"}`+"\n"+`{"provider":"openai-codex"}`+"\n", now.Add(-1*time.Minute))
	// recent session on anthropic
	writeSession(t, d, "--E--b--/s2.jsonl", `{"provider":"anthropic"}`+"\n", now.Add(-2*time.Minute))
	// old session on google: outside the window
	writeSession(t, d, "--E--c--/s3.jsonl", `{"provider":"google"}`+"\n", now.Add(-time.Hour))

	got := piActiveProviders(d, now, 5*time.Minute)
	if !got["claude"] || !got["codex"] || got["gemini"] {
		t.Errorf("got %v, want claude and codex only", got)
	}
}

func TestPiSessionDirsActivity(t *testing.T) {
	now := time.Date(2026, 11, 6, 16, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeSession(t, root, "sessions/parent/session.jsonl", `{"provider":"anthropic"}`+"\n", now)
	writeSession(t, root, "gentle-agents/sessions/child/session.jsonl", `{"provider":"openai-codex"}`+"\n", now.Add(-time.Minute))
	writeSession(t, root, "gentle-agents/sessions/old/session.jsonl", `{"provider":"google"}`+"\n", now.Add(-6*time.Minute))

	active := map[string]bool{}
	for _, dir := range piSessionDirs([]string{root, filepath.Join(root, "missing")}) {
		for card := range piActiveProviders(dir, now, 5*time.Minute) {
			active[card] = true
		}
	}
	if len(active) != 2 || !active["claude"] || !active["codex"] {
		t.Fatalf("got %v, want only claude parent and codex child", active)
	}
}

func TestPiActiveProvidersMissingDir(t *testing.T) {
	if got := piActiveProviders(filepath.Join(t.TempDir(), "nope"), time.Now(), 5*time.Minute); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestPiProviderCard(t *testing.T) {
	cases := map[string]string{"anthropic": "claude", "openai-codex": "codex", "openai": "codex", "google": "gemini", "google-gemini-cli": "gemini", "deepseek": ""}
	for in, want := range cases {
		if got := piProviderCard(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestPiAgentDirs(t *testing.T) {
	home := filepath.Join("h")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	want := []string{filepath.Join("h", ".gentle-shell", "agent"), filepath.Join("h", ".pi", "agent")}
	if got := piAgentDirs(home); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("default: got %v want %v", got, want)
	}
	// a custom Pi agent dir is added first; duplicates are dropped
	t.Setenv("PI_CODING_AGENT_DIR", want[1])
	if got := piAgentDirs(home); len(got) != 2 || got[0] != want[1] || got[1] != want[0] {
		t.Errorf("env dup: got %v", got)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "custom")
	if got := piAgentDirs(home); len(got) != 3 || got[0] != "custom" {
		t.Errorf("env custom: got %v", got)
	}
}
