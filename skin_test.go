package main

import (
	"os"
	"strings"
	"testing"
)

// Guards the skin refresh chain (docs/design.md "Refresh chain"). Each of
// these broke the widget once: it froze on its load-time values, or ran two
// exe instances at once.
func TestSkinRefreshChain(t *testing.T) {
	raw, err := os.ReadFile("skin/AgentMeter/AgentMeter.ini")
	if err != nil {
		t.Fatal(err)
	}
	ini := string(raw)

	for _, want := range []string{
		"FinishAction=[!UpdateMeasure MeasureData]",
		"UpdateRate=1",
	} {
		if !strings.Contains(ini, want) {
			t.Errorf("skin is missing %q", want)
		}
	}
	for _, bad := range []string{
		`!CommandMeasure MeasureData "Update"`,        // does not re-fetch: skin freezes
		"OnRefreshAction=[!CommandMeasure MeasureRun", // second concurrent exe run
	} {
		if strings.Contains(ini, bad) {
			t.Errorf("skin must not contain %q", bad)
		}
	}
}
