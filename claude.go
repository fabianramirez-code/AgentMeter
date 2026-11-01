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
// (RFC3339 string). If this provider shows "error", the URL, headers or
// response schema likely changed. Last verified: 2026-10-31.

type claudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
		ExpiresAt   int64  `json:"expiresAt"` // unix millis
	} `json:"claudeAiOauth"`
}

type claudeUsage struct {
	FiveHour claudeWindow `json:"five_hour"`
	SevenDay claudeWindow `json:"seven_day"`
}

type claudeWindow struct {
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resets_at"`
}

// parseClaudeUsage parses the /api/oauth/usage response body.
func parseClaudeUsage(body []byte) (claudeUsage, error) {
	var u claudeUsage
	if err := json.Unmarshal(body, &u); err != nil {
		return claudeUsage{}, fmt.Errorf("parse claude usage: %w", err)
	}
	return u, nil
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
