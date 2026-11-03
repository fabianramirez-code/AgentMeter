# AgentMeter — notes for agents and contributors

Read `docs/design.md` before changing the skin or the poll logic.

## Skin (`skin/AgentMeter/AgentMeter.ini`)

- Install with `.\build.ps1`, then Refresh in Rainmeter. Never hand-copy the
  `.ini`: Rainmeter needs UTF-16 LE + BOM, the repo keeps UTF-8.
- Refresh chain is `MeasureClock -> MeasureRun -> MeasureData.FinishAction`.
  `MeasureRun.FinishAction` must contain ONLY `[!CommandMeasure MeasureData "Update"]`.
  Every `!UpdateMeasureGroup DataChildren` / `!UpdateMeterGroup Cards` / `!Redraw`
  that depends on fresh data goes in `MeasureData.FinishAction`. WebParser parses
  asynchronously; updating earlier shows stale values until a manual Refresh.
  See "Refresh chain" in `docs/design.md`.
- Data measures (`Group=DataChildren`) keep `UpdateDivider=-1`; they are only
  updated from `MeasureData.FinishAction`.
- Container meters: the origin is the meter's own X/Y, not the Shape offset.

## Collector (`*.go`)

- Network fetches are gated by `shouldFetch` in `pollgate.go`: min 5 min between
  attempts, fallback `-fallback` minutes. Do not poll on every run; the Claude
  endpoint answers 429.
- `go test ./...` must pass; `build.ps1` runs it before building.
