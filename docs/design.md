# AgentMeter — Design

Rainmeter desktop widget showing subscription quota usage for Claude, Codex and Gemini (Antigravity CLI).

## Architecture

No daemon. The Rainmeter skin runs a one-shot Go executable every 60 s (RunCommand plugin).
If the skin is not loaded, nothing runs.

```
AgentMeter.ini ── every UpdateRate ──> agentmeter.exe (runs ~100 ms, exits)
                                         reads  state.json   (cache between runs)
                                         writes data.json    (read by WebParser)
                                         writes state.json
```

## Data sources

| Provider | Source | Windows shown | Notes |
|---|---|---|---|
| Codex | latest `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`, last `rate_limits` | primary (300 min), secondary (10080 min) | Local file, no network. |
| Claude | `GET https://api.anthropic.com/api/oauth/usage`, bearer from `~/.claude/.credentials.json` (`claudeAiOauth.accessToken`) | five_hour, seven_day | Undocumented endpoint. |
| Gemini | `POST https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels`, bearer from Windows Credential Manager target `gemini:antigravity` | weekly remaining per model group | Undocumented endpoint. Remaining is converted to used = 100 - remaining. |

None of these consume model tokens: they are account metadata reads.

## Poll gating (per network provider)

```
every run:
  stale cache older than FallbackMinutes (default 20)  -> fetch
  process running AND activity mtime changed           -> fetch
  otherwise                                            -> reuse cached values
  always: recompute "resets in Xh Ym"; if resets_at passed -> used = 0
```

- Process list: `CreateToolhelp32Snapshot` (golang.org/x/sys/windows). Names configurable.
- Activity: Claude = newest mtime under `~/.claude/projects`; Gemini = newest mtime under `~/.gemini/antigravity-cli/conversations`.
- Fallback poll covers usage from web/mobile, which shares quota and leaves no local trace.

## Credentials and sharing

- No secret is ever stored in code, config, `data.json` or `state.json`.
- Tokens are read at runtime from each user's own CLI login; colleagues just need to be logged into the CLIs.
- Expired token: do NOT refresh (would race the CLI's own token rotation); mark provider `stale` and keep last values.
- Any failure (missing login, HTTP error, schema change) degrades only that provider to `status: "error"` with a short message.

## Skin

- ~320x160 base size multiplied by `Scale`; background alpha from `WindowAlpha`.
- Header: `AgentMeter / <Provider>`, ▲ ▼ cycle enabled providers, ⚙ toggles an in-skin settings panel.
- Settings panel: provider checkboxes, transparency −/+, size −/+. Persisted with `!WriteKeyValue`.
- Cards: colored thin border, glowing dot, big %, rounded progress bar, reset subtext.
- All config (enabled providers, process names, interval) lives in the skin `[Variables]` and is passed to the exe as flags.

### Refresh chain (do not reorder)

```
MeasureClock (Calc, UpdateDivider=60)
  -> [!CommandMeasure MeasureRun "Run"]            runs agentmeter.exe, writes data.json
MeasureRun.FinishAction
  -> [!UpdateMeasure MeasureData]                   ONLY this bang, nothing else
MeasureData.FinishAction   (fires when the parse has finished)
  -> [!UpdateMeasureGroup DataChildren][!UpdateMeterGroup Cards][!Redraw]
```

`MeasureClock` also fires on its first update, so it is the load-time run too;
an extra `OnRefreshAction` Run would start a second exe at the same time
(RunCommand logs "Error 101: Program still running").

Use `!UpdateMeasure MeasureData`, not `!CommandMeasure MeasureData "Update"`:
the latter does not trigger a fetch (checked with `Debug=2`: no `Fetching:`
line), so the skin showed the load-time values forever. `UpdateRate=1` makes
every `!UpdateMeasure` re-read the file.

Why the split: WebParser parses `data.json` in a background thread.
If `MeasureRun.FinishAction` also updates `DataChildren`/`Cards`, those bangs
run before the parse is done and read the previous values. The children have
`UpdateDivider=-1`, so nothing re-reads them until a manual Refresh: the skin
looks frozen while the exe keeps writing fresh data. Anything that must react
to new data (new child measures, new meters, Lua calls) hangs off
`MeasureData.FinishAction`, never off `MeasureRun.FinishAction`.

Two cadences, easy to confuse when "it does not update":

| What | Every |
|---|---|
| exe run + skin redraw + "resets in" countdown | 60 s (`MeasureClock`) |
| network fetch per provider | min 5 min (`minFetchInterval`), fallback `FallbackMinutes` (20) |

Percentages only change when a fetch happens and usage actually moved.

## macOS and Linux

Same collector, same `data.json`. OS-specific code lives behind build tags:
`win.go`, `darwin.go` and `linux.go` each provide `isProcessRunning`,
`readCredentialBlob`, `readClaudeCredentials` and `defaultProcs`;
`newestModTime` (`activity.go`) and `decodeKeyringSecret` (`keyring.go`) are
portable.

| | macOS | Linux |
|---|---|---|
| Processes | `sysctl kern.proc.all`, first 16 bytes (`MAXCOMLEN`) | `/proc/<pid>/comm`, first 15 bytes (`TASK_COMM_LEN`) |
| Credentials | `/usr/bin/security find-generic-password -w -s service -a account` | `secret-tool lookup service S username A` |
| Claude | keychain first, then the file | the file |

A Windows target `"service:account"` maps to those service/account pairs,
which is how go-keyring names items on each OS; its `go-keyring-base64:` /
`go-keyring-encoded:` prefixes are decoded.

Front ends (`unix/`), no Rainmeter refresh chain involved:

```
SwiftBar / Argos / Kargos (agentmeter.1m.sh) ─ run.sh xbar   ─┐
Waybar custom module ───────────────────────── run.sh waybar ─┤
Conky execpi ───────────────────────────────── run.sh conky  ─┼─> ~/.agentmeter/run.sh
Übersicht (index.jsx) ──────────────────────── run.sh        ─┘     skip if data.json < 50 s old
                                                                    mkdir lock: one agentmeter at a time
                                                                    agentmeter -render <format>  (or cat data.json)
```

The lock matters when several front ends are installed: without it two runs
could read the same `state.json`, both pass `shouldFetch` and double the API
calls. A reader that finds the lock taken waits (max ~5 s) so it never reads
a half-written `data.json`.

`-render` (`render.go`) only reads `data.json`; it never collects or touches
the network. Keep provider colors and the 60/85 % traffic light in sync with
the skin `[Variables]` and the Übersicht widget.
