package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// gentle-shell (Pi) runs every model from one node.exe, so its process name
// says nothing about which provider is in use. Instead, each session is a
// JSONL file under ~/.gentle-shell/agent/sessions/<project>/ whose message
// lines carry "provider":"<id>". A session file written within the window
// marks the card of its latest provider as active.

var piProviderRe = regexp.MustCompile(`"provider":"([^"]+)"`)

// piTailBytes is how much of the end of a session file is scanned for the
// latest provider; session files grow to hundreds of KB.
const piTailBytes = 64 << 10

// piProviderCard maps a Pi provider id to an AgentMeter card name, or "".
func piProviderCard(provider string) string {
	switch {
	case provider == "anthropic":
		return "claude"
	case provider == "openai" || strings.HasPrefix(provider, "openai-"):
		return "codex"
	case provider == "google" || strings.HasPrefix(provider, "google-"):
		return "gemini"
	}
	return ""
}

// piActiveProviders returns the cards whose provider was used by a Pi
// session file modified within window of now.
func piActiveProviders(sessionsDir string, now time.Time, window time.Duration) map[string]bool {
	active := map[string]bool{}
	_ = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil || now.Sub(info.ModTime()) > window {
			return nil
		}
		if card := piProviderCard(lastPiProvider(path)); card != "" {
			active[card] = true
		}
		return nil
	})
	return active
}

// lastPiProvider returns the last "provider" value in the tail of a session file.
func lastPiProvider(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > piTailBytes {
		f.Seek(info.Size()-piTailBytes, io.SeekStart)
	}
	tail, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	m := piProviderRe.FindAllSubmatch(tail, -1)
	if len(m) == 0 {
		return ""
	}
	return string(m[len(m)-1][1])
}

// piAgentDirs lists the Pi agent directories to read auth.json and sessions
// from: $PI_CODING_AGENT_DIR when set, gentle-shell's ~/.gentle-shell/agent
// and plain Pi's ~/.pi/agent. Missing directories are harmless.
func piAgentDirs(home string) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, d := range []string{os.Getenv("PI_CODING_AGENT_DIR"), filepath.Join(home, ".gentle-shell", "agent"), filepath.Join(home, ".pi", "agent")} {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// piPaths joins name onto every Pi agent dir.
func piPaths(dirs []string, name string) []string {
	out := make([]string, len(dirs))
	for i, d := range dirs {
		out[i] = filepath.Join(d, name)
	}
	return out
}
