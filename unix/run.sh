#!/bin/sh
# Shared entry point for every macOS/Linux front end, installed as
# ~/.agentmeter/run.sh:
#
#   run.sh           print data.json (Übersicht)
#   run.sh xbar      SwiftBar, Argos, Kargos
#   run.sh waybar    Waybar custom module
#   run.sh conky     Conky execpi
#
# Runs agentmeter at most once per 50 s and one at a time, even when several
# front ends call it every minute. Extra agentmeter flags (e.g.
# -providers claude,codex) can go in ~/.agentmeter/flags.
dir="$(cd "$(dirname "$0")" && pwd)"
data="$dir/data.json"
lock="$dir/.lock"
format="${1:-}"

# GNU stat first: BSD stat rejects -c, while GNU stat -f means something else.
mtime() { stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || echo 0; }
now=$(date +%s)

# A lock older than 60 s belongs to a run that was killed; take it over.
if [ -d "$lock" ] && [ $((now - $(mtime "$lock"))) -gt 60 ]; then
	rmdir "$lock" 2>/dev/null
fi

if [ $((now - $(mtime "$data"))) -ge 50 ] && mkdir "$lock" 2>/dev/null; then
	trap 'rmdir "$lock"' EXIT
	# shellcheck disable=SC2046 # flags are meant to word-split
	"$dir/agentmeter" -out "$data" -state "$dir/state.json" -log "$dir/agentmeter.log" \
		$(cat "$dir/flags" 2>/dev/null) 2>/dev/null
	rmdir "$lock"
	trap - EXIT
else
	# Another front end is running it right now: wait (max ~5 s) so we never
	# read a half-written data.json.
	i=0
	while [ -d "$lock" ] && [ $i -lt 25 ]; do
		sleep 0.2
		i=$((i + 1))
	done
fi

if [ -n "$format" ]; then
	exec "$dir/agentmeter" -render "$format" -out "$data" -log "$dir/agentmeter.log"
fi
cat "$data"
