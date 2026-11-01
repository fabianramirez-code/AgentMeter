package main

import (
	"testing"
	"time"
)

func TestParseGeminiCredential(t *testing.T) {
	// Shape verified against the real Windows Credential Manager blob for
	// target "gemini:antigravity" (token value here is a fixture, not real).
	blob := `{"token":{"access_token":"fixture-token-value","token_type":"Bearer","refresh_token":"fixture-refresh","expiry":"2026-10-31T23:30:37.5374436-06:00","expires_in":3599},"auth_method":"oauth","id_token_email":"user@example.com"}`

	token, expiry, err := parseGeminiCredential([]byte(blob))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "fixture-token-value" {
		t.Errorf("token = %q, want fixture-token-value", token)
	}
	want, _ := time.Parse(time.RFC3339Nano, "2026-10-31T23:30:37.5374436-06:00")
	if !expiry.Equal(want) {
		t.Errorf("expiry = %v, want %v", expiry, want)
	}
}

func TestParseGeminiCredentialMissingToken(t *testing.T) {
	_, _, err := parseGeminiCredential([]byte(`{"token":{"expiry":"2026-10-31T23:30:37-06:00"}}`))
	if err == nil {
		t.Fatal("expected error for missing access_token")
	}
}

// realGeminiModelsBody mirrors the real fetchAvailableModels response shape
// (model ids trimmed, quota values adjusted to exercise grouping).
const realGeminiModelsBody = `{"models":{
  "gpt-oss-120b-medium": {"modelProvider":"MODEL_PROVIDER_OPENAI","apiProvider":"API_PROVIDER_OPENAI_VERTEX","quotaInfo":{"remainingFraction":0.9,"resetTime":"2026-11-08T05:18:02Z"}},
  "claude-opus-4-6-thinking": {"modelProvider":"MODEL_PROVIDER_ANTHROPIC","apiProvider":"API_PROVIDER_ANTHROPIC_VERTEX","quotaInfo":{"remainingFraction":0.65,"resetTime":"2026-11-09T05:18:02Z"}},
  "gemini-2.5-pro": {"modelProvider":"MODEL_PROVIDER_GOOGLE","apiProvider":"API_PROVIDER_GOOGLE_GEMINI","quotaInfo":{"remainingFraction":0.8,"resetTime":"2026-11-01T10:18:02Z"}},
  "gemini-2.5-flash": {"modelProvider":"MODEL_PROVIDER_GOOGLE","apiProvider":"API_PROVIDER_GOOGLE_GEMINI","quotaInfo":{"remainingFraction":0.3,"resetTime":"2026-11-01T11:00:00Z"}},
  "chat_23310": {"modelProvider":"MODEL_PROVIDER_GOOGLE","apiProvider":"API_PROVIDER_INTERNAL","quotaInfo":{"remainingFraction":1}}
}}`

func TestGroupGeminiUsage(t *testing.T) {
	gemini, other, err := groupGeminiUsage([]byte(realGeminiModelsBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Gemini group: worst model is gemini-2.5-flash at 0.3 remaining -> 70% used.
	if gemini.UsedPct != 70 {
		t.Errorf("gemini.UsedPct = %d, want 70", gemini.UsedPct)
	}
	wantGeminiReset, _ := time.Parse(time.RFC3339, "2026-11-01T11:00:00Z")
	if !gemini.Reset.Equal(wantGeminiReset) {
		t.Errorf("gemini.Reset = %v, want %v", gemini.Reset, wantGeminiReset)
	}

	// Other group: worst model is claude-opus at 0.65 remaining -> 35% used.
	if other.UsedPct != 35 {
		t.Errorf("other.UsedPct = %d, want 35", other.UsedPct)
	}
	wantOtherReset, _ := time.Parse(time.RFC3339, "2026-11-09T05:18:02Z")
	if !other.Reset.Equal(wantOtherReset) {
		t.Errorf("other.Reset = %v, want %v", other.Reset, wantOtherReset)
	}
}

func TestGroupGeminiUsageMissingGroup(t *testing.T) {
	body := `{"models":{"gemini-2.5-pro":{"modelProvider":"MODEL_PROVIDER_GOOGLE","quotaInfo":{"remainingFraction":0.8,"resetTime":"2026-11-01T10:18:02Z"}}}}`
	_, _, err := groupGeminiUsage([]byte(body))
	if err == nil {
		t.Fatal("expected error when the Claude/GPT-OSS group has no models")
	}
}

func TestGroupGeminiUsageInvalidJSON(t *testing.T) {
	_, _, err := groupGeminiUsage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
