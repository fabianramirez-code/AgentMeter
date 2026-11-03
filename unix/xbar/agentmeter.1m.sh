#!/bin/sh
# <swiftbar.title>AgentMeter</swiftbar.title>
# <swiftbar.desc>Claude, Codex and Gemini quota usage.</swiftbar.desc>
# <swiftbar.hideRunInTerminal>true</swiftbar.hideRunInTerminal>
# <swiftbar.hideDisablePlugin>true</swiftbar.hideDisablePlugin>
#
# Menu bar plugin in xbar format, for SwiftBar (macOS), Argos (GNOME) and
# Kargos (KDE). "1m" in the file name is the refresh interval. The menu is
# rendered by agentmeter itself (see render.go), so this stays a one-liner.
exec "$HOME/.agentmeter/run.sh" xbar
