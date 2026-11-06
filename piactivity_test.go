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
