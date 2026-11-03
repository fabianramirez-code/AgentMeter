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
  -> [!CommandMeasure MeasureData "Update"]         ONLY this bang, nothing else
MeasureData.FinishAction   (fires when the parse has finished)
  -> [!UpdateMeasureGroup DataChildren][!UpdateMeterGroup Cards][!Redraw]
```

Why the split: WebParser `Update` parses `data.json` in a background thread.
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
