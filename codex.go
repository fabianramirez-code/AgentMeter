package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// NOTE: undocumented jsonl schema used by Codex CLI session rollouts.
// If this provider shows "error", the rollout line shape under
// payload.rate_limits (primary/secondary.{used_percent,resets_at}) likely
// changed. Last verified: 2026-10-31.

type codexRateLimits struct {
	PrimaryPct     float64
	PrimaryReset   time.Time
	SecondaryPct   float64
	SecondaryReset time.Time
}

type codexRolloutLine struct {
	Payload struct {
		RateLimits *struct {
			Primary struct {
				UsedPercent float64 `json:"used_percent"`
				ResetsAt    int64   `json:"resets_at"`
			} `json:"primary"`
			Secondary struct {
				UsedPercent float64 `json:"used_percent"`
				ResetsAt    int64   `json:"resets_at"`
			} `json:"secondary"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

// parseCodexLine parses one jsonl line and extracts rate limit info.
// Returns an error if the line has no rate_limits payload.
func parseCodexLine(line []byte) (codexRateLimits, error) {
	var l codexRolloutLine
	if err := json.Unmarshal(line, &l); err != nil {
		return codexRateLimits{}, fmt.Errorf("parse codex line: %w", err)
	}
	if l.Payload.RateLimits == nil {
		return codexRateLimits{}, fmt.Errorf("no rate_limits in line")
	}
	rl := l.Payload.RateLimits
	return codexRateLimits{
		PrimaryPct:     rl.Primary.UsedPercent,
		PrimaryReset:   time.Unix(rl.Primary.ResetsAt, 0),
		SecondaryPct:   rl.Secondary.UsedPercent,
		SecondaryReset: time.Unix(rl.Secondary.ResetsAt, 0),
	}, nil
}

// findLatestRateLimits walks sessionsDir/YYYY/MM/DD (newest first) looking for the
// newest rollout-*.jsonl file, then returns the last line in it that contains rate_limits.
func findLatestRateLimits(sessionsDir string) (codexRateLimits, error) {
	file, err := newestRolloutFile(sessionsDir)
	if err != nil {
		return codexRateLimits{}, err
	}
	line, err := lastRateLimitsLine(file)
	if err != nil {
		return codexRateLimits{}, err
	}
	return parseCodexLine(line)
}

// newestRolloutFile finds the most recently modified rollout-*.jsonl file under
// sessionsDir, scanning date directories newest-first to avoid walking the whole tree.
func newestRolloutFile(sessionsDir string) (string, error) {
	var best string
	var bestMod time.Time

	years, err := sortedSubdirsDesc(sessionsDir)
	if err != nil {
		return "", err
	}
	for _, y := range years {
		months, err := sortedSubdirsDesc(filepath.Join(sessionsDir, y))
		if err != nil {
			continue
		}
		for _, m := range months {
			days, err := sortedSubdirsDesc(filepath.Join(sessionsDir, y, m))
			if err != nil {
				continue
			}
			for _, d := range days {
				dayDir := filepath.Join(sessionsDir, y, m, d)
				entries, err := os.ReadDir(dayDir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if e.IsDir() || !isRolloutName(e.Name()) {
						continue
					}
					info, err := e.Info()
					if err != nil {
						continue
					}
					if info.ModTime().After(bestMod) {
						bestMod = info.ModTime()
						best = filepath.Join(dayDir, e.Name())
					}
				}
				if best != "" {
					return best, nil // found in the newest day with any rollout file
				}
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no rollout files found under %s", sessionsDir)
	}
	return best, nil
}

func isRolloutName(name string) bool {
	return len(name) > len("rollout-") && name[:len("rollout-")] == "rollout-" && filepath.Ext(name) == ".jsonl"
}

// sortedSubdirsDesc returns the immediate subdirectory names of dir, sorted descending.
func sortedSubdirsDesc(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

// lastRateLimitsLine returns the last line in file that contains "rate_limits".
func lastRateLimitsLine(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var last []byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for sc.Scan() {
		b := sc.Bytes()
		if containsRateLimits(b) {
			last = append([]byte{}, b...)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if last == nil {
		return nil, fmt.Errorf("no rate_limits line found in %s", path)
	}
	return last, nil
}

func containsRateLimits(b []byte) bool {
	return bytes.Contains(b, []byte(`"rate_limits"`))
}
