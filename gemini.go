package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NOTE: undocumented endpoint used by the Antigravity CLI (agy) for /usage.
// POST https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels
// with header User-Agent: antigravity-cli (plain Bearer auth alone gets a 403;
// the x-goog-user-project header also 403s - the User-Agent is what the real
// CLI sends and what the backend checks). Body {}. Response has a "models" map
// keyed by model id, each with modelProvider/apiProvider and an optional
// quotaInfo.{remainingFraction,resetTime}. Grouped into "Gemini" (Google Gemini
// models) and "Claude/GPT-OSS" (Anthropic/OpenAI models hosted via Antigravity),
// using the worst (lowest remainingFraction) model per group. If this provider
// shows "error", the URL, header, or response schema likely changed.
// Last verified: 2026-10-31.

// NOTE: undocumented Windows Credential Manager target "gemini:antigravity"
// (generic credential, UTF-8 JSON blob) used by the Antigravity CLI. Verified
// shape: {"token":{"access_token":str,"refresh_token":str,"expiry":RFC3339 str,
// "expires_in":int,"token_type":str},"auth_method":str,"id_token_email":str}.
// If this provider shows "error", the credential target name or blob schema
// likely changed. Last verified: 2026-10-31.

type geminiCredential struct {
	Token struct {
		AccessToken string `json:"access_token"`
		Expiry      string `json:"expiry"` // RFC3339Nano
	} `json:"token"`
}

// parseGeminiCredential parses the Credential Manager blob (UTF-8 JSON).
func parseGeminiCredential(blob []byte) (token string, expiry time.Time, err error) {
	var c geminiCredential
	if err := json.Unmarshal(blob, &c); err != nil {
		return "", time.Time{}, fmt.Errorf("parse gemini credential: %w", err)
	}
	if c.Token.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("gemini credential has no access_token")
	}
	t, err := time.Parse(time.RFC3339Nano, c.Token.Expiry)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse gemini token expiry: %w", err)
	}
	return c.Token.AccessToken, t, nil
}

type geminiQuotaInfo struct {
	RemainingFraction float64   `json:"remainingFraction"`
	ResetTime         time.Time `json:"resetTime"`
}

type geminiModel struct {
	ModelProvider string           `json:"modelProvider"`
	QuotaInfo     *geminiQuotaInfo `json:"quotaInfo"`
}

type geminiModelsResponse struct {
	Models map[string]geminiModel `json:"models"`
}

type geminiGroup struct {
	UsedPct int
	Reset   time.Time
}

// groupGeminiUsage parses fetchAvailableModels' response and reduces it to the
// two quota groups the skin shows: "Gemini" (Google models) and "Claude/GPT-OSS"
// (third-party models hosted through Antigravity). Within each group the model
// with the least remaining quota wins, since that is the binding constraint.
func groupGeminiUsage(body []byte) (gemini geminiGroup, other geminiGroup, err error) {
	var resp geminiModelsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return geminiGroup{}, geminiGroup{}, fmt.Errorf("parse gemini models: %w", err)
	}

	var haveGemini, haveOther bool
	bestGeminiFrac := 2.0
	bestOtherFrac := 2.0

	for _, m := range resp.Models {
		if m.QuotaInfo == nil || m.QuotaInfo.ResetTime.IsZero() {
			continue // internal/placeholder models with no real quota pool
		}
		switch m.ModelProvider {
		case "MODEL_PROVIDER_GOOGLE":
			if m.QuotaInfo.RemainingFraction < bestGeminiFrac {
				bestGeminiFrac = m.QuotaInfo.RemainingFraction
				gemini.Reset = m.QuotaInfo.ResetTime
				haveGemini = true
			}
		case "MODEL_PROVIDER_ANTHROPIC", "MODEL_PROVIDER_OPENAI":
			if m.QuotaInfo.RemainingFraction < bestOtherFrac {
				bestOtherFrac = m.QuotaInfo.RemainingFraction
				other.Reset = m.QuotaInfo.ResetTime
				haveOther = true
			}
		}
	}

	if !haveGemini {
		return geminiGroup{}, geminiGroup{}, fmt.Errorf("no Gemini models with quota info found")
	}
	if !haveOther {
		return geminiGroup{}, geminiGroup{}, fmt.Errorf("no Claude/GPT-OSS models with quota info found")
	}

	gemini.UsedPct = int((1 - bestGeminiFrac) * 100)
	other.UsedPct = int((1 - bestOtherFrac) * 100)
	return gemini, other, nil
}

// fetchGeminiModels calls the undocumented fetchAvailableModels endpoint. Thin network I/O, untested.
func fetchGeminiModels(accessToken string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels", strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity-cli")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini fetchAvailableModels returned status %d", resp.StatusCode)
	}
	return body, nil
}
