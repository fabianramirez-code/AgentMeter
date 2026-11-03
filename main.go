package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// result is one provider's output row for data.json. Never contains secrets.
type result struct {
	Status  string // ok|stale|error|off
	Msg     string
	C1Label string
	C1Pct   int
	C1Reset string
	C2Label string
	C2Pct   int
	C2Reset string
	C3Label string
	C3Pct   int
	C3Reset string
}

func main() {
	outPath := flag.String("out", "data.json", "path to write data.json")
	statePath := flag.String("state", "state.json", "path to the state cache")
	providersFlag := flag.String("providers", "claude,codex,gemini", "comma-separated enabled providers")
	procsFlag := flag.String("procs", defaultProcs, "provider=processName pairs separated by ;")
	fallbackMin := flag.Int("fallback", 20, "fallback poll interval in minutes")
	logPath := flag.String("log", "", "append fetches and errors to this file (empty = stderr only)")
	flag.Parse()

	log.SetPrefix("agentmeter: ")
	if *logPath != "" {
		if f, err := openLog(*logPath, maxLogBytes, time.Now()); err != nil {
			log.Println("cannot open log:", err)
		} else {
			defer f.Close()
			log.SetOutput(io.MultiWriter(os.Stderr, f))
		}
	}

	enabled := map[string]bool{}
	for _, p := range strings.Split(*providersFlag, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			enabled[p] = true
		}
	}
	procs := parsePairs(*procsFlag)
	fallback := time.Duration(*fallbackMin) * time.Minute

	home, err := os.UserHomeDir()
	if err != nil {
		log.Println("cannot resolve home dir:", err)
		home = "."
	}

	now := time.Now()
	st := loadState(*statePath)

	out := map[string]string{}
	for _, name := range []string{"claude", "codex", "gemini"} {
		var r result
		if !enabled[name] {
			r = result{Status: "off"}
		} else {
			r = runProvider(name, now, home, procs[name], fallback, st)
			if apiCall := st.Providers[name].LastAttempt.Equal(now); apiCall || r.Status != "ok" {
				log.Printf("%s: status=%s api_call=%v msg=%q", name, r.Status, apiCall, r.Msg)
			}
		}
		addResult(out, name, r)
		// "1" when the provider's CLI process is running; drives the card dot.
		out[name+"_active"] = "0"
		if enabled[name] && isProcessRunning(procs[name]) {
			out[name+"_active"] = "1"
		}
	}

	if err := saveState(*statePath, st); err != nil {
		log.Println("failed to save state:", err)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Println("failed to marshal output:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		log.Println("failed to write output:", err)
		os.Exit(1)
	}
}

// maxLogBytes caps the log; past it the file is renamed with a timestamp
// (agentmeter.log -> agentmeter_20261102234411.log) and a new one starts.
// ponytail: rotated files are never pruned; only fetches/errors are logged
// (~1 MB per month), add pruning if they pile up.
const maxLogBytes = 1 << 20

func openLog(path string, maxBytes int64, now time.Time) (*os.File, error) {
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxBytes {
		ext := filepath.Ext(path)
		rotated := strings.TrimSuffix(path, ext) + now.Format("_20060102150405") + ext
		os.Rename(path, rotated) // best effort; on failure keep appending
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

func parsePairs(s string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(s, ";") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

func addResult(out map[string]string, name string, r result) {
	out[name+"_status"] = r.Status
	out[name+"_msg"] = r.Msg
	out[name+"_c1_label"] = r.C1Label
	out[name+"_c1_pct"] = fmt.Sprintf("%d", r.C1Pct)
	out[name+"_c1_reset"] = r.C1Reset
	out[name+"_c2_label"] = r.C2Label
	out[name+"_c2_pct"] = fmt.Sprintf("%d", r.C2Pct)
	out[name+"_c2_reset"] = r.C2Reset
	out[name+"_c3_label"] = r.C3Label
	out[name+"_c3_pct"] = fmt.Sprintf("%d", r.C3Pct)
	out[name+"_c3_reset"] = r.C3Reset
}

// runProvider updates st.Providers[name] in place (caching the result) and
// returns the row to render this run. Each provider fails independently.
func runProvider(name string, now time.Time, home, procName string, fallback time.Duration, st state) result {
	cached := st.Providers[name]

	switch name {
	case "codex":
		ps, r := runCodex(now, filepath.Join(home, ".codex", "sessions"), cached)
		st.Providers[name] = ps
		return r
	case "claude":
		ps, r := runClaude(now, filepath.Join(home, ".claude", ".credentials.json"), procName, filepath.Join(home, ".claude", "projects"), fallback, cached)
		st.Providers[name] = ps
		return r
	case "gemini":
		ps, r := runGemini(now, procName, filepath.Join(home, ".gemini", "antigravity-cli", "conversations"), fallback, cached)
		st.Providers[name] = ps
		return r
	default:
		return result{Status: "error", Msg: "unknown provider"}
	}
}

func runCodex(now time.Time, sessionsDir string, cached providerState) (providerState, result) {
	rl, err := findLatestRateLimits(sessionsDir)
	if err != nil {
		if cached.LastFetch.IsZero() {
			return cached, errorResult("error", "no Codex session with rate_limits found")
		}
		return cached, cardsToResult("stale", "no Codex session, cached values", now, cached)
	}

	ps := providerState{
		LastFetch: now,
		C1:        cardState{Label: "SESSION (5H)", UsedPct: int(rl.PrimaryPct), ResetAt: rl.PrimaryReset},
		C2:        cardState{Label: "WEEKLY (7D)", UsedPct: int(rl.SecondaryPct), ResetAt: rl.SecondaryReset},
	}
	return ps, cardsToResult("ok", "", now, ps)
}

func runClaude(now time.Time, credPath, procName, activityDir string, fallback time.Duration, cached providerState) (providerState, result) {
	raw, err := readClaudeCredentials(credPath)
	if err != nil {
		return cached, errorResult("error", "not logged into Claude")
	}
	var creds claudeCredentials
	if err := json.Unmarshal(raw, &creds); err != nil || creds.ClaudeAiOauth.AccessToken == "" {
		return cached, errorResult("error", "could not read Claude credentials")
	}

	if isTokenExpired(now, creds.ClaudeAiOauth.ExpiresAt) {
		if cached.LastFetch.IsZero() {
			return cached, errorResult("stale", "login expired, open claude")
		}
		return cached, cardsToResult("stale", "login expired, open claude", now, cached)
	}

	processRunning := isProcessRunning(procName)
	activity := newestModTime(activityDir)
	activityChanged := !activity.Equal(cached.LastActivity) && !activity.IsZero()

	if !shouldFetch(now, cached.LastAttempt, cached.LastFetch, minFetchInterval, fallback, processRunning, activityChanged) {
		if cached.LastFetch.IsZero() {
			// a recent attempt failed and nothing is cached yet; wait for minFetchInterval.
			return cached, errorResult("error", "no cached Claude data yet, retrying soon")
		}
		return cached, cardsToResult("ok", "", now, cached)
	}

	cached.LastAttempt = now
	usage, err := fetchClaudeUsage(creds.ClaudeAiOauth.AccessToken)
	if err != nil {
		log.Println("claude: fetch failed:", err)
		if cached.LastFetch.IsZero() {
			return cached, errorResult("error", "request failed: "+err.Error())
		}
		return cached, cardsToResult("stale", "request failed, cached values", now, cached)
	}

	ps := providerState{
		LastFetch:    now,
		LastAttempt:  now,
		LastActivity: activity,
		C1:           cardState{Label: "SESSION (5H)", UsedPct: int(usage.FiveHour.Utilization), ResetAt: usage.FiveHour.ResetsAt},
		C2:           cardState{Label: "WEEKLY (7D)", UsedPct: int(usage.SevenDay.Utilization), ResetAt: usage.SevenDay.ResetsAt},
	}
	if scoped, displayName, ok := scopedWeekly(usage); ok {
		ps.C3 = cardState{Label: "WEEKLY " + strings.ToUpper(displayName) + " (7D)", UsedPct: int(scoped.Utilization), ResetAt: scoped.ResetsAt}
	}
	return ps, cardsToResult("ok", "", now, ps)
}

func runGemini(now time.Time, procName, activityDir string, fallback time.Duration, cached providerState) (providerState, result) {
	blob, err := readCredentialBlob("gemini:antigravity")
	if err != nil {
		return cached, errorResult("error", "not logged into Antigravity (Gemini)")
	}
	token, expiry, err := parseGeminiCredential(blob)
	if err != nil {
		return cached, errorResult("error", "could not read Antigravity credentials")
	}

	if !expiry.After(now) {
		if cached.LastFetch.IsZero() {
			return cached, errorResult("stale", "login expired, open agy")
		}
		return cached, cardsToResult("stale", "login expired, open agy", now, cached)
	}

	processRunning := isProcessRunning(procName)
	activity := newestModTime(activityDir)
	activityChanged := !activity.Equal(cached.LastActivity) && !activity.IsZero()

	if !shouldFetch(now, cached.LastAttempt, cached.LastFetch, minFetchInterval, fallback, processRunning, activityChanged) {
		if cached.LastFetch.IsZero() {
			return cached, errorResult("error", "no cached Gemini data yet, retrying soon")
		}
		return cached, cardsToResult("ok", "", now, cached)
	}

	cached.LastAttempt = now
	body, err := fetchGeminiModels(token)
	if err != nil {
		log.Println("gemini: fetch failed:", err)
		if cached.LastFetch.IsZero() {
			return cached, errorResult("error", "request failed: "+err.Error())
		}
		return cached, cardsToResult("stale", "request failed, cached values", now, cached)
	}
	geminiGroup, otherGroup, err := groupGeminiUsage(body)
	if err != nil {
		if cached.LastFetch.IsZero() {
			return cached, errorResult("error", "Gemini usage response: "+err.Error())
		}
		return cached, cardsToResult("stale", "response changed, cached values", now, cached)
	}

	ps := providerState{
		LastFetch:    now,
		LastAttempt:  now,
		LastActivity: activity,
		C1:           cardState{Label: "GEMINI (7D)", UsedPct: geminiGroup.UsedPct, ResetAt: geminiGroup.Reset},
		C2:           cardState{Label: "CLAUDE/GPT-OSS (7D)", UsedPct: otherGroup.UsedPct, ResetAt: otherGroup.Reset},
	}
	return ps, cardsToResult("ok", "", now, ps)
}

// cardsToResult renders a providerState's two cards into a result row,
// recomputing reset text and post-reset zeroing against now on every run.
// The skin has no separate status/message meter, so a non-ok status is
// folded into the reset text directly: "(stale)" appended when a reset
// countdown is still available, or the message alone otherwise.
func cardsToResult(status, msg string, now time.Time, ps providerState) result {
	r1 := withStatusSuffix(status, msg, formatReset(now, ps.C1.ResetAt))
	r2 := withStatusSuffix(status, msg, formatReset(now, ps.C2.ResetAt))
	r := result{
		Status:  status,
		Msg:     msg,
		C1Label: ps.C1.Label,
		C1Pct:   effectiveUsedPct(now, ps.C1.ResetAt, ps.C1.UsedPct),
		C1Reset: r1,
		C2Label: ps.C2.Label,
		C2Pct:   effectiveUsedPct(now, ps.C2.ResetAt, ps.C2.UsedPct),
		C2Reset: r2,
	}
	if ps.C3.Label != "" {
		r.C3Label = ps.C3.Label
		r.C3Pct = effectiveUsedPct(now, ps.C3.ResetAt, ps.C3.UsedPct)
		r.C3Reset = withStatusSuffix(status, msg, formatReset(now, ps.C3.ResetAt))
	}
	return r
}

func withStatusSuffix(status, msg, reset string) string {
	if status == "ok" || msg == "" {
		return reset
	}
	if reset == "" {
		return msg
	}
	return reset + " (" + status + ")"
}

// errorResult is used on the very first run for a provider (nothing cached
// yet), where there is no reset countdown to show, so the message itself
// becomes the reset-line text for both cards.
func errorResult(status, msg string) result {
	return result{Status: status, Msg: msg, C1Reset: msg, C2Reset: msg}
}
