package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// Text renderers for the macOS/Linux front ends (-render). They read the
// data.json written by a normal run, so every front end shows the same
// cards and none needs a JSON parser of its own (no jq, no Python).
//
//	xbar    SwiftBar (macOS), Argos (GNOME), Kargos (KDE): menu bar item + menu
//	waybar  Waybar custom module: one JSON line, cards in the tooltip
//	conky   Conky execpi: conky text with colors, all providers stacked

type renderProvider struct{ id, name, color string }

// Same accents and traffic light as the Rainmeter skin [Variables].
var renderProviders = []renderProvider{
	{"claude", "Claude", "E8664A"},
	{"codex", "Codex", "2EBFA5"},
	{"gemini", "Gemini", "4A90E2"},
}

const (
	warnPct     = 60
	critPct     = 85
	colorWarn   = "F2C94C"
	colorCrit   = "E85046"
	colorOk     = "CDCDD4"
	colorActive = "4CC978"
	colorIdle   = "6E6E76"
	colorDim    = "96969E"
)

type renderCard struct {
	label string
	pct   int
	reset string
}

type providerView struct {
	renderProvider
	status, msg string
	active      bool
	cards       []renderCard
}

func runRender(format, dataPath, logPath string, w io.Writer) error {
	var render func(io.Writer, []providerView, string)
	var renderErr func(io.Writer, string)
	switch format {
	case "xbar":
		render, renderErr = renderXbar, renderXbarError
	case "waybar":
		render, renderErr = func(w io.Writer, vs []providerView, _ string) { renderWaybar(w, vs) }, renderWaybarError
	case "conky":
		render, renderErr = func(w io.Writer, vs []providerView, _ string) { renderConky(w, vs) }, renderConkyError
	default:
		return fmt.Errorf("unknown -render format %q (want xbar, waybar or conky)", format)
	}

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		renderErr(w, "no data yet: "+err.Error())
		return nil
	}
	var d map[string]string
	if err := json.Unmarshal(raw, &d); err != nil {
		renderErr(w, "data.json is not valid JSON")
		return nil
	}
	render(w, providerViews(d), logPath)
	return nil
}

// providerViews returns the enabled providers, in display order.
func providerViews(d map[string]string) []providerView {
	var vs []providerView
	for _, p := range renderProviders {
		status := d[p.id+"_status"]
		if status == "" || status == "off" {
			continue
		}
		v := providerView{renderProvider: p, status: status, msg: d[p.id+"_msg"], active: d[p.id+"_active"] == "1"}
		for _, c := range []string{"c1", "c2", "c3"} {
			label := d[p.id+"_"+c+"_label"]
			if label == "" {
				continue
			}
			var pct int
			fmt.Sscan(d[p.id+"_"+c+"_pct"], &pct)
			v.cards = append(v.cards, renderCard{label: label, pct: pct, reset: d[p.id+"_"+c+"_reset"]})
		}
		vs = append(vs, v)
	}
	return vs
}

// maxPct is the highest usage across all cards, or -1 when there are none.
func maxPct(vs []providerView) int {
	m := -1
	for _, v := range vs {
		for _, c := range v.cards {
			m = max(m, c.pct)
		}
	}
	return m
}

// level is the traffic light for a usage %, thresholds inclusive.
func level(pct int) string {
	switch {
	case pct >= critPct:
		return "crit"
	case pct >= warnPct:
		return "warn"
	default:
		return "ok"
	}
}

func levelColor(pct int) string {
	switch level(pct) {
	case "crit":
		return colorCrit
	case "warn":
		return colorWarn
	default:
		return colorOk
	}
}

func textBar(pct, width int) string {
	filled := min(max(pct, 0), 100) * width / 100
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

// --- xbar (SwiftBar / Argos / Kargos) ---

// xbarText makes s safe for one xbar line: "|" starts the parameters.
func xbarText(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "|", "/")), " ")
}

// xbarColor: SwiftBar takes "light,dark" pairs, so warn/crit stay readable
// on a light menu; Argos and Kargos take a single color.
func xbarColor(pct int) string {
	if level(pct) == "ok" {
		return ""
	}
	c := "#" + levelColor(pct)
	if runtime.GOOS == "darwin" {
		dark := map[string]string{"warn": "#B7791F", "crit": "#C53030"}[level(pct)]
		c = dark + "," + c
	}
	return " color=" + c
}

func renderXbar(w io.Writer, vs []providerView, logPath string) {
	mac := runtime.GOOS == "darwin"
	font, icon, open := "monospace", "iconName=utilities-system-monitor-symbolic", "bash=xdg-open param1="
	if mac {
		font, icon, open = "Menlo", "sfimage=gauge.medium", "bash=/usr/bin/open param1=-t param2="
	}

	title, m := "AgentMeter", maxPct(vs)
	if m >= 0 {
		title = fmt.Sprintf("%d%%", m)
	}
	color := ""
	if m >= 0 {
		color = xbarColor(m)
	}
	fmt.Fprintf(w, "%s | %s%s\n", title, icon, color)

	for _, v := range vs {
		running := ""
		if v.active {
			running = "  ● running"
		}
		fmt.Fprintf(w, "---\n%s%s | color=#%s size=14\n", v.name, running, v.color)
		for _, c := range v.cards {
			fmt.Fprintf(w, "%-22s%3d%%  %s | font=%s size=12%s\n", xbarText(c.label), c.pct, textBar(c.pct, 12), font, xbarColor(c.pct))
			if reset := xbarText(c.reset); reset != "" {
				fmt.Fprintf(w, "%s | font=%s size=11\n", reset, font)
			}
		}
		if len(v.cards) == 0 {
			fmt.Fprintf(w, "%s | size=12 color=#%s\n", xbarText(firstNonEmpty(v.msg, v.status)), colorCrit)
		}
	}
	fmt.Fprintf(w, "---\nReload | refresh=true\nOpen log | %s%q terminal=false\n", open, logPath)
}

func renderXbarError(w io.Writer, msg string) {
	fmt.Fprintf(w, "AgentMeter ⚠ | color=#%s\n---\n%s | size=12\n---\nReload | refresh=true\n", colorCrit, xbarText(msg))
}

// --- Waybar ---

type waybarOutput struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class"`
	Percentage int    `json:"percentage"`
}

// Waybar tooltips are Pango markup.
var pangoEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func renderWaybar(w io.Writer, vs []providerView) {
	var tip []string
	for _, v := range vs {
		head := fmt.Sprintf(`<span color="#%s"><b>%s</b></span>`, v.color, v.name)
		if v.active {
			head += fmt.Sprintf(` <span color="#%s">●</span>`, colorActive)
		}
		tip = append(tip, head)
		for _, c := range v.cards {
			line := fmt.Sprintf(`<tt>%-22s<span color="#%s">%3d%%</span></tt>  %s`,
				pangoEscaper.Replace(c.label), levelColor(c.pct), c.pct, pangoEscaper.Replace(c.reset))
			tip = append(tip, strings.TrimRight(line, " "))
		}
		if len(v.cards) == 0 {
			tip = append(tip, pangoEscaper.Replace(firstNonEmpty(v.msg, v.status)))
		}
	}
	m := maxPct(vs)
	out := waybarOutput{Text: "AgentMeter", Tooltip: strings.Join(tip, "\n"), Class: "ok"}
	if m >= 0 {
		out = waybarOutput{Text: fmt.Sprintf("%d%%", m), Tooltip: out.Tooltip, Class: level(m), Percentage: m}
	}
	json.NewEncoder(w).Encode(out)
}

func renderWaybarError(w io.Writer, msg string) {
	json.NewEncoder(w).Encode(waybarOutput{Text: "AgentMeter ⚠", Tooltip: pangoEscaper.Replace(msg), Class: "error"})
}

// --- Conky ---

// "$" starts a conky variable; "$$" is a literal dollar.
func conkyText(s string) string { return strings.ReplaceAll(s, "$", "$$") }

func renderConky(w io.Writer, vs []providerView) {
	for i, v := range vs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		dot := colorIdle
		if v.active {
			dot = colorActive
		}
		fmt.Fprintf(w, "${color %s}${font Sans:bold:size=11}%s${font}${color}${alignr}${color %s}●${color}\n", v.color, v.name, dot)
		for _, c := range v.cards {
			col := levelColor(c.pct)
			fmt.Fprintf(w, "%s${alignr}${color %s}${font Sans:bold:size=11}%d%%${font}${color}\n", conkyText(c.label), col, c.pct)
			fmt.Fprintf(w, "${color %s}%s${color}\n", col, textBar(c.pct, 28))
			if c.reset != "" {
				fmt.Fprintf(w, "${color %s}%s${color}\n", colorDim, conkyText(c.reset))
			}
		}
		if len(v.cards) == 0 {
			fmt.Fprintf(w, "${color %s}%s${color}\n", colorCrit, conkyText(firstNonEmpty(v.msg, v.status)))
		}
	}
}

func renderConkyError(w io.Writer, msg string) {
	fmt.Fprintf(w, "${color %s}%s${color}\n", colorCrit, conkyText(msg))
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
