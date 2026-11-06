# AgentMeter

A Rainmeter desktop widget that shows subscription quota usage for Claude,
Codex and Gemini (Antigravity CLI): session (5h) and weekly (7d) usage, time
until reset, and whether each CLI is currently running.

Works with the official CLIs (Claude Code, Codex CLI, Antigravity CLI) and
with the [Pi](https://github.com/earendil-works/pi) coding agent and
gentle-shell (a Pi distribution): see [Pi and gentle-shell](#pi-and-gentle-shell).

On the Claude screen, if your account has a per-model weekly limit (shown as
"Current week (<model>)" in `claude /usage`, e.g. Fable), a third card appears
below the usual two. The widget doesn't grow to fit it: the card area scrolls
with the mouse wheel instead, with a thin scrollbar on the right edge.
Codex and Gemini never have a third card, so their screens never scroll.

No daemon: the skin runs a small Go executable every 60 seconds. It reads
each CLI's own local login, writes a flat `data.json`, and exits. If the
skin is not loaded, nothing runs.

## Screenshots

<p align="center">
  <img src="img/1.png" width="600" alt="AgentMeter widget on the desktop"><br>
  <img src="img/2.png" width="260" alt="Session and weekly usage cards">
  <img src="img/3.png" width="260" alt="Settings: providers, transparency, size">
</p>
<p align="center">
  <img src="img/4.png" width="260" alt="Expired login state">
  <img src="img/5.png" width="260" alt="Codex provider card">
  <img src="img/6.png" width="260" alt="Active process indicator">
</p>

## Requirements

| What | Needed for | Notes |
|---|---|---|
| Windows 10/11 | everything | Uses the Windows process list and Credential Manager. |
| [Go](https://go.dev/dl/) 1.26+ | building `agentmeter.exe` | **Optional to install**: if `go` is not on `PATH`, `build.ps1` downloads the official zip into `.\go`. |
| PowerShell 7 (`pwsh`) or Windows PowerShell | running `build.ps1` | Both work. |
| [Rainmeter](https://www.rainmeter.net/) 4.5+ | showing the widget | **Optional to install**: `build.ps1 -Portable` downloads a portable copy into the project. |

Plus a login in each CLI you want tracked. Untracked or logged-out providers
simply show an error message on their cards; the others keep working.

| Provider | Log in with | Where AgentMeter reads it |
|---|---|---|
| Claude | Claude Code (`claude`), or Pi / gentle-shell `/login` with a Claude subscription (used when Claude Code's token is missing or expired) | `%USERPROFILE%\.claude\.credentials.json`, then the Pi `auth.json` |
| Codex | Codex CLI (`codex`), or Pi / gentle-shell `/login` with ChatGPT (`openai-codex`) | `%USERPROFILE%\.codex\sessions\...\rollout-*.jsonl`, or the usage endpoint with the token in the Pi `auth.json`; the newer wins |
| Gemini | Antigravity CLI (`agy`) | Windows Credential Manager, target `gemini:antigravity` |

## Pi and gentle-shell

[Pi](https://github.com/earendil-works/pi) and gentle-shell run every model
from one `node.exe`, so AgentMeter reads their agent directory instead of a
process name. It checks, in order: `%PI_CODING_AGENT_DIR%` (if set),
`%USERPROFILE%\.gentle-shell\agent` (gentle-shell) and
`%USERPROFILE%\.pi\agent` (plain Pi).

- **Log in with your subscription**, not an API key: run `/login` and pick
  Anthropic (Claude Pro/Max) and/or ChatGPT (`openai-codex`). The token is
  stored in `auth.json` and Pi refreshes it while you use it; AgentMeter only
  reads it. An API key (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) is billed per
  token and has no subscription quota, so the cards cannot show it.
- **Claude**: uses the Pi token when Claude Code's is missing or expired, so
  you no longer need to open Claude Code to keep the card fresh.
- **Codex**: polls the ChatGPT usage endpoint with the Pi token; if you also
  run the `codex` CLI, the newer value wins.
- **Gemini**: not supported through Pi, which has no Antigravity login; the
  card still needs the `agy` CLI login.
- **Polling and the dot**: a Pi session file (`sessions\**\*.jsonl`) written
  in the last 5 minutes counts as activity for the provider of its latest
  message (`anthropic` -> Claude, `openai*` -> Codex, `google*` -> Gemini):
  that card's dot turns green and it is polled every 5 minutes instead of
  every fallback interval.

## Install

Quickest, no Go needed: download `AgentMeter-<version>.zip` from
[Releases](https://github.com/fabianramirez-code/AgentMeter/releases), extract
the `AgentMeter` folder into `%USERPROFILE%\Documents\Rainmeter\Skins\`, then
in Rainmeter: **Refresh all**, **Manage**, `AgentMeter\AgentMeter.ini`,
**Load**. The bundled `agentmeter.exe` is not code-signed, so Windows
SmartScreen or your antivirus may ask before the first run.

To build from source instead:

Option A, you already have Rainmeter installed:

```powershell
.\build.ps1
```

Then in Rainmeter: right-click the tray icon, **Manage**, select
`AgentMeter\AgentMeter.ini`, **Load**.

Option B, nothing installed (no Go, no Rainmeter):

```powershell
.\build.ps1 -Portable
```

On the first run this downloads the official Rainmeter installer from
GitHub releases, checks its signature, and installs it in portable mode into
`.\rainmeter` (git-ignored; no registry or Start menu changes). It then
installs the skin there and launches it. Later, start it with
`.\rainmeter\Rainmeter.exe`.

Both options use Go from `PATH` if present. Otherwise, on the first run they
download the latest official Go zip from go.dev, verify its SHA256 checksum,
and extract it into `.\go` (git-ignored; no installer, no admin rights, no
system `PATH` change). Then they run `go test ./...` and build
`agentmeter.exe`. Re-running
`build.ps1` updates the skin and resets its saved settings to the defaults.

## Using the widget

- **▲ / ▼** switch between enabled providers. The choice is remembered.
- **⚙** opens settings: enable/disable providers, transparency (0-100 %),
  size (0.75x-2x). The panel stays open until you click **Done**.
- **Card color** is a usage traffic light: gray-white below 60 %, yellow from
  60 %, red from 85 %. Change `WarnPct` / `CritPct` in `AgentMeter.ini`.
- **Dot** (top-right of each card): green means that provider's CLI
  process is running right now, or a gentle-shell session used that
  provider in the last 5 minutes (`anthropic` -> Claude, `openai*` -> Codex,
  `google*` -> Gemini, read from the Pi agent `sessions` folder); gray
  means neither.
- Process names are configurable in `AgentMeter.ini` (`ProcClaude`,
  `ProcCodex`, `ProcGemini`).

## macOS and Linux

The collector also builds on macOS and Linux. Rainmeter is Windows-only, so
there are other front ends; one installer picks the ones that fit:

```bash
unix/install.sh
```

| System | Front end | What you get |
|---|---|---|
| macOS | [SwiftBar](https://swiftbar.app/) | menu bar item: highest usage %, traffic-light colored; the menu lists every card |
| macOS | [Übersicht](https://tracesof.net/uebersicht/) | desktop widget with the skin's cards; ▲ / ▼ switch providers |
| GNOME | [Argos](https://extensions.gnome.org/extension/1176/argos/) | top bar item, same menu as SwiftBar |
| KDE Plasma | [Kargos](https://github.com/lipido/kargos) | panel item, same menu as SwiftBar |
| Hyprland, Sway, ... | [Waybar](https://github.com/Alexays/Waybar) | bar module: highest % in the bar, cards in the tooltip (see `unix/waybar/README.md`) |
| any other (XFCE, Cinnamon, i3, ...) | [Conky](https://github.com/brndnmtthws/conky) | desktop panel with every provider stacked, started at login |

The installer runs `go test ./...`, builds `agentmeter` into `~/.agentmeter`
(Go from `PATH`, or a checksum-verified project-local copy in `./go`), reads
`XDG_CURRENT_DESKTOP` on Linux, installs what it can and prints what is left
to do by hand (e.g. installing the Argos extension, or adding the Waybar
module to your config, which it never edits). On macOS, install SwiftBar
and/or Übersicht and launch each once before running it (SwiftBar asks for a
plugin folder).

Every front end calls `~/.agentmeter/run.sh`, which runs `agentmeter` at most
once per 50 s and one at a time, so several front ends never multiply the API
calls. Menus and panels are rendered by `agentmeter -render xbar|waybar|conky`,
so none needs jq or Python. Extra flags (e.g. `-providers claude,codex`) go in
`~/.agentmeter/flags`; data, state and log live in the same folder.

Differences from Windows:

| Provider | macOS | Linux |
|---|---|---|
| Claude | login keychain, service `Claude Code-credentials` (falls back to `~/.claude/.credentials.json`) | `~/.claude/.credentials.json`, as on Windows |
| Codex | same session files as on Windows | same session files as on Windows |
| Gemini | login keychain, service `gemini`, account `antigravity` (**not yet verified**) | Secret Service via `secret-tool` (`libsecret-tools`), service `gemini`, username `antigravity` (**not yet verified**) |

macOS may ask once whether `security` can read each keychain item. Process
names default to `claude`, `codex` and `agy` (no `.exe`); a Claude Code
installed through npm runs as `node` and is not detected as running.

## Network usage

Codex is read from a local file, plus one HTTPS request when you are logged
into gentle-shell with ChatGPT. Claude and Gemini need one HTTPS request
each, and these requests do **not** consume model tokens (they read account
metadata, like the CLIs' own `/usage` command). To stay well below the
endpoints' rate limits, a provider is polled only when:

- it has never been fetched, or
- its CLI is running **and** its local logs changed since the last fetch, or
- the last fetch is older than 20 minutes (catches web/mobile usage),

and never more than once every 5 minutes, even after a failed attempt.
Countdowns ("resets in 2h 14m") are recomputed locally every run.

## Sharing and credentials

No credentials live in this repository, in the built executable, or in
`data.json` / `state.json`. At runtime `agentmeter.exe` reads tokens from the
current user's own CLI login and never stores, logs, or refreshes them.
Sharing the repo shares code only; each person sees their own usage.

## Why might a provider show "error" or "stale"?

Each provider fails independently.

- **Claude** uses the undocumented `GET https://api.anthropic.com/api/oauth/usage`
  endpoint. If Anthropic changes its URL, headers, or `five_hour`/`seven_day`
  response shape, Claude shows `error`.
- **Codex** reads `payload.rate_limits` from the newest session file. If the
  Codex CLI changes that schema, Codex shows `error`. With a gentle-shell
  ChatGPT login it also calls the undocumented
  `GET https://chatgpt.com/backend-api/wham/usage`; if that changes, Codex
  shows `stale` and falls back to the session files.
- **Gemini** reads the Antigravity token from Credential Manager and calls the
  undocumented `fetchAvailableModels` endpoint on
  `daily-cloudcode-pa.googleapis.com`, sending `User-Agent: antigravity-cli`
  (without it the server answers 403). Changes to the credential layout,
  endpoint, header check, or `quotaInfo` shape show `error`.
- **stale / "login expired"**: the CLI's token expired. AgentMeter never
  refreshes tokens itself (that could race the CLI's own rotation). Open that
  CLI once and the data comes back on the next poll. Antigravity only
  refreshes its token while `agy` is running.
- **"request failed"** after many reloads: likely HTTP 429 from the endpoint;
  it recovers on its own after the 5-minute minimum interval.

Every one of these call sites has a `NOTE:` comment in the source
(`claude.go`, `codex.go`, `gemini.go`) saying what to check first.

Every network fetch, non-ok status and fetch error (with the real error text)
is appended to `@Resources\agentmeter.log` in the installed skin. Past 1 MB it
is renamed to `agentmeter_<yyyyMMddHHmmss>.log` and a new one starts.
Skin-side problems (meters not redrawing) show up in Rainmeter's own log
instead: right-click the tray icon, About, Log.

## Project layout

```
main.go, claude.go, codex.go, gemini.go   collector (one file per provider)
pollgate.go, state.go, format.go          polling rules, cache, reset text
win.go, darwin.go, linux.go               OS APIs (processes, credentials)
activity.go, keyring.go, render.go        log mtimes, keyring decoding, Unix front-end output
*_test.go                                 go test ./...
skin/AgentMeter/AgentMeter.ini            Rainmeter skin (UTF-8 here, UTF-16 when installed)
skin/AgentMeter/@Resources/Scripts/       provider cycling and settings panel (Lua)
build.ps1                                 test, build, install (optionally portable)
unix/                                     install.sh, run.sh and the macOS/Linux front ends
go/, rainmeter/                           local toolchains downloaded by the installers (git-ignored)
docs/design.md                            design notes
```

### Editing the skin

Always install skin changes with `.\build.ps1`, never by copying
`AgentMeter.ini` into the Rainmeter `Skins` folder by hand. The repo keeps
the `.ini` as plain UTF-8 (diff-friendly), but Rainmeter reads skin files as
ANSI unless they carry a UTF-16 LE BOM, so a raw copy turns the header glyphs
(▲ ▼ ⚙) into mojibake. `build.ps1` does that conversion at install time.
It does not reload the skin, so afterwards run **Refresh** in Rainmeter (or
`Rainmeter.exe !Refresh AgentMeter`) to see the change.

## License

AgentMeter is MIT licensed (see `LICENSE`). Rainmeter is GPL v2 and is
downloaded from its official releases, not
redistributed in this repository.
