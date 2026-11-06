package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NOTE: undocumented endpoint used by the Claude Code CLI for /usage.
// GET https://api.anthropic.com/api/oauth/usage with headers
// Authorization: Bearer <claudeAiOauth.accessToken> and
// anthropic-beta: oauth-2025-04-20. Verified response has five_hour and
// seven_day objects, each with "utilization" (0-100 float) and "resets_at"
// (RFC3339 string). It also has a "limits" array of per-window entries,
// each with "kind" ("session", "weekly_all", "weekly_scoped", ...),
// "percent" (0-100 int), "resets_at" (RFC3339 string), and "scope"
// (null, or {"model": {"display_name": "Fable"}, "surface": ...} for a
// weekly_scoped entry). A "weekly_scoped" entry is the per-model weekly
// limit (e.g. "Fable"); there is no top-level seven_day_fable key. If this
// provider shows "error", the URL, headers or response schema likely
// changed. Last verified: 2026-11-02.

type claudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
		ExpiresAt   int64  `json:"expiresAt"` // unix millis
	} `json:"claudeAiOauth"`
}

type claudeUsage struct {
	FiveHour claudeWindow  `json:"five_hour"`
	SevenDay claudeWindow  `json:"seven_day"`
	Limits   []claudeLimit `json:"limits"`
}

type claudeWindow struct {
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resets_at"`
}

type claudeLimit struct {
	Kind     string    `json:"kind"`
	Percent  int       `json:"percent"`
	ResetsAt time.Time `json:"resets_at"`
	Scope    *struct {
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// scopedWeekly finds the first "weekly_scoped" entry in u.Limits (the
// per-model weekly limit, e.g. "Fable") and returns it as a claudeWindow
// plus its display name. ok is false if there is no such entry.
func scopedWeekly(u claudeUsage) (claudeWindow, string, bool) {
	for _, l := range u.Limits {
		if l.Kind == "weekly_scoped" && l.Scope != nil {
			return claudeWindow{Utilization: float64(l.Percent), ResetsAt: l.ResetsAt}, l.Scope.Model.DisplayName, true
		}
	}
	return claudeWindow{}, "", false
}

// parseClaudeUsage parses the /api/oauth/usage response body.
func parseClaudeUsage(body []byte) (claudeUsage, error) {
	var u claudeUsage
	if err := json.Unmarshal(body, &u); err != nil {
		return claudeUsage{}, fmt.Errorf("parse claude usage: %w", err)
	}
	return u, nil
}

// claudeToken is one Claude subscription OAuth access token and its expiry.
type claudeToken struct {
	AccessToken string
	ExpiresAt   int64 // unix millis
}

// parsePiAnthropicToken reads the Anthropic subscription token that
// gentle-shell (Pi) stores in ~/.gentle-shell/agent/auth.json after /login:
// {"anthropic":{"type":"oauth","access":"...","expires":<unix millis>}}.
// Pi refreshes it itself while in use; we only read it. An api_key entry is
// rejected: the usage endpoint answers 400 for API organizations.
func parsePiAnthropicToken(raw []byte) (claudeToken, error) {
	var auth map[string]struct {
		Type    string `json:"type"`
		Access  string `json:"access"`
		Expires int64  `json:"expires"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil {
		return claudeToken{}, fmt.Errorf("parse pi auth: %w", err)
	}
	a, ok := auth["anthropic"]
	if !ok || a.Type != "oauth" || a.Access == "" {
		return claudeToken{}, fmt.Errorf("pi auth has no anthropic oauth login")
	}
	return claudeToken{AccessToken: a.Access, ExpiresAt: a.Expires}, nil
}

// pickClaudeToken returns the first candidate that has not expired.
func pickClaudeToken(now time.Time, candidates []claudeToken) (claudeToken, bool) {
	for _, c := range candidates {
		if c.AccessToken != "" && !isTokenExpired(now, c.ExpiresAt) {
			return c, true
		}
	}
	return claudeToken{}, false
}

// isTokenExpired reports whether a millisecond-epoch expiry has passed.
func isTokenExpired(now time.Time, expiresAtMillis int64) bool {
	return now.UnixMilli() >= expiresAtMillis
}

// fetchClaudeUsage calls the undocumented usage endpoint. Thin network I/O, untested.
func fetchClaudeUsage(accessToken string) (claudeUsage, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return claudeUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return claudeUsage{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return claudeUsage{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return claudeUsage{}, fmt.Errorf("claude usage endpoint returned status %d", resp.StatusCode)
	}
	return parseClaudeUsage(body)
}
