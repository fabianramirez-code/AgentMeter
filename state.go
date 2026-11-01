package main

import (
	"encoding/json"
	"os"
	"time"
)

// cardState is the cached data for one usage window (e.g. Claude's five_hour,
// or a Codex/Gemini quota group). Never contains secrets.
type cardState struct {
	Label      string    `json:"label"`
	UsedPct    int       `json:"used_pct"`
	ResetAt    time.Time `json:"reset_at"`
}

// providerState is the per-provider cache persisted to state.json between runs.
type providerState struct {
	LastFetch    time.Time `json:"last_fetch"`
	LastAttempt  time.Time `json:"last_attempt"`
	LastActivity time.Time `json:"last_activity"`
	C1           cardState `json:"c1"`
	C2           cardState `json:"c2"`
}

// state is the full state.json contents. Never contains secrets - only
// timestamps and usage percentages.
type state struct {
	Providers map[string]providerState `json:"providers"`
}

func loadState(path string) state {
	s := state{Providers: map[string]providerState{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s) // best-effort; corrupt cache just means a fresh fetch
	if s.Providers == nil {
		s.Providers = map[string]providerState{}
	}
	return s
}

func saveState(path string, s state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
