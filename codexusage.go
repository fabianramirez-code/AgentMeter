package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// NOTE: undocumented endpoint used by the Codex CLI for /status.
// GET https://chatgpt.com/backend-api/wham/usage with headers
// Authorization: Bearer <access> and chatgpt-account-id: <accountId>.
// Verified response has rate_limit.primary_window (5h) and
// rate_limit.secondary_window (7d), each with "used_percent" (0-100) and
// "reset_at" (unix seconds). The token is the ChatGPT login gentle-shell (Pi)
// stores in ~/.gentle-shell/agent/auth.json under "openai-codex" after
// /login; Pi refreshes it, we only read it. Last verified: 2026-11-05.

type codexToken struct {
	AccessToken string
	AccountID   string
	ExpiresAt   int64 // unix millis
}

// parsePiCodexToken reads the openai-codex OAuth login from Pi's auth.json.
func parsePiCodexToken(raw []byte) (codexToken, error) {
	var auth map[string]struct {
		Type      string `json:"type"`
		Access    string `json:"access"`
		Expires   int64  `json:"expires"`
		AccountID string `json:"accountId"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil {
		return codexToken{}, fmt.Errorf("parse pi auth: %w", err)
	}
	a, ok := auth["openai-codex"]
	if !ok || a.Type != "oauth" || a.Access == "" || a.AccountID == "" {
		return codexToken{}, fmt.Errorf("pi auth has no openai-codex oauth login")
	}
	return codexToken{AccessToken: a.Access, AccountID: a.AccountID, ExpiresAt: a.Expires}, nil
}

// readPiCodexToken returns the gentle-shell ChatGPT token if present and unexpired.
func readPiCodexToken(now time.Time, path string) (codexToken, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return codexToken{}, false
	}
	tok, err := parsePiCodexToken(raw)
	if err != nil || isTokenExpired(now, tok.ExpiresAt) {
		return codexToken{}, false
	}
	return tok, true
}

// parseCodexUsage parses the /backend-api/wham/usage response body.
func parseCodexUsage(body []byte) (codexRateLimits, error) {
	type window struct {
		UsedPercent float64 `json:"used_percent"`
		ResetAt     int64   `json:"reset_at"`
	}
	var u struct {
		RateLimit *struct {
			Primary   *window `json:"primary_window"`
			Secondary *window `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return codexRateLimits{}, fmt.Errorf("parse codex usage: %w", err)
	}
	if u.RateLimit == nil || u.RateLimit.Primary == nil || u.RateLimit.Secondary == nil {
		return codexRateLimits{}, fmt.Errorf("codex usage has no rate_limit windows")
	}
	return codexRateLimits{
		PrimaryPct:     u.RateLimit.Primary.UsedPercent,
		PrimaryReset:   time.Unix(u.RateLimit.Primary.ResetAt, 0),
		SecondaryPct:   u.RateLimit.Secondary.UsedPercent,
		SecondaryReset: time.Unix(u.RateLimit.Secondary.ResetAt, 0),
	}, nil
}

// fetchCodexUsage calls the undocumented usage endpoint. Thin network I/O, untested.
func fetchCodexUsage(tok codexToken) (codexRateLimits, error) {
	req, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		return codexRateLimits{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("chatgpt-account-id", tok.AccountID)
	req.Header.Set("User-Agent", "agentmeter")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return codexRateLimits{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return codexRateLimits{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return codexRateLimits{}, fmt.Errorf("codex usage endpoint returned status %d", resp.StatusCode)
	}
	return parseCodexUsage(body)
}
