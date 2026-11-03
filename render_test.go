package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleData() map[string]string {
	return map[string]string{
		"claude_status": "ok", "claude_active": "1",
		"claude_c1_label": "SESSION (5H)", "claude_c1_pct": "23", "claude_c1_reset": "resets 4:50pm in 4h 19m",
		"claude_c2_label": "WEEKLY (7D)", "claude_c2_pct": "88", "claude_c2_reset": "resets Fri 5am | $soon",
		"codex_status":  "off",
		"gemini_status": "error", "gemini_msg": "not logged into Antigravity (Gemini)",
	}
}

func render(t *testing.T, format string, d map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.json")
	if d != nil {
		raw, _ := json.Marshal(d)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := runRender(format, path, "/tmp/agentmeter.log", &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestProviderViewsSkipsOffAndEmptyCards(t *testing.T) {
	vs := providerViews(sampleData())
	if len(vs) != 2 || vs[0].id != "claude" || vs[1].id != "gemini" {
		t.Fatalf("views = %+v, want claude and gemini", vs)
	}
	if len(vs[0].cards) != 2 || len(vs[1].cards) != 0 {
		t.Errorf("cards: claude=%d gemini=%d, want 2 and 0", len(vs[0].cards), len(vs[1].cards))
	}
	if got := maxPct(vs); got != 88 {
		t.Errorf("maxPct = %d, want 88", got)
	}
}

func TestLevelThresholdsInclusive(t *testing.T) {
	for pct, want := range map[int]string{0: "ok", 59: "ok", 60: "warn", 84: "warn", 85: "crit", 100: "crit"} {
		if got := level(pct); got != want {
			t.Errorf("level(%d) = %q, want %q", pct, got, want)
		}
	}
}

func TestRenderXbar(t *testing.T) {
	out := render(t, "xbar", sampleData())
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "88% | ") {
		t.Errorf("title = %q, want the max %% first", lines[0])
	}
	if strings.Contains(out, "Codex") {
		t.Error("disabled provider must not be listed")
	}
	for _, want := range []string{"Claude  ● running", "SESSION (5H)", "not logged into Antigravity (Gemini)", "refresh=true"} {
		if !strings.Contains(out, want) {
			t.Errorf("xbar output is missing %q:\n%s", want, out)
		}
	}
	// A "|" inside a value would start the parameter list.
	if !strings.Contains(out, "resets Fri 5am / $soon | ") {
		t.Errorf("reset text not sanitized:\n%s", out)
	}
}

func TestRenderWaybar(t *testing.T) {
	var got waybarOutput
	if err := json.Unmarshal([]byte(render(t, "waybar", sampleData())), &got); err != nil {
		t.Fatal(err)
	}
	if got.Text != "88%" || got.Class != "crit" || got.Percentage != 88 {
		t.Errorf("waybar = %+v, want 88%% crit", got)
	}
	if !strings.Contains(got.Tooltip, "SESSION (5H)") || strings.Contains(got.Tooltip, "Codex") {
		t.Errorf("tooltip = %q", got.Tooltip)
	}
}

func TestRenderConkyEscapesDollar(t *testing.T) {
	out := render(t, "conky", sampleData())
	if !strings.Contains(out, "$$soon") {
		t.Errorf("a literal $ must be doubled for conky:\n%s", out)
	}
	if !strings.Contains(out, "${color E8664A}") {
		t.Errorf("provider accent missing:\n%s", out)
	}
}

func TestRenderMissingDataShowsError(t *testing.T) {
	for _, format := range []string{"xbar", "waybar", "conky"} {
		if out := render(t, format, nil); !strings.Contains(out, "no data yet") {
			t.Errorf("%s with no data.json = %q, want an error message", format, out)
		}
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	if err := runRender("nope", "data.json", "", &bytes.Buffer{}); err == nil {
		t.Error("want an error for an unknown format")
	}
}
