#!/bin/sh
# Usage:
#   unix/install.sh   test and build agentmeter into ~/.agentmeter, then set up
#                     the front ends that fit this machine:
#
#   macOS   SwiftBar (menu bar) and/or Übersicht (desktop), whichever is set up
#   Linux   GNOME -> Argos, KDE Plasma -> Kargos, Waybar (Hyprland, Sway, ...),
#           Conky as the desktop panel when no bar front end fits
#
# Re-running it updates everything; data.json/state.json are kept.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$HOME/.agentmeter"
os="$(uname -s)"
done_msgs=""
todo_msgs=""
ok() { done_msgs="$done_msgs\n  ✓ $1"; }
todo() { todo_msgs="$todo_msgs\n  • $1"; }

# --- Go ---------------------------------------------------------------------
# Use Go from PATH; otherwise use (or download once) a project-local copy in
# ./go (git-ignored). It is the official tarball, checksum-verified: no
# installer, no root, no system PATH changes.
go="$(command -v go || true)"
local_go="$root/go/bin/go"
if [ -z "$go" ] && [ ! -x "$local_go" ]; then
	echo "Go not found, downloading a project-local copy..."
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) echo "Unsupported CPU $(uname -m); install Go 1.26+ and re-run." >&2 && exit 1 ;;
	esac
	goos="$(echo "$os" | tr '[:upper:]' '[:lower:]')"
	file="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1).$goos-$arch.tar.gz"
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	curl -fsSL -o "$tmp/$file" "https://dl.google.com/go/$file"
	want="$(curl -fsSL "https://dl.google.com/go/$file.sha256")"
	got="$( (sha256sum "$tmp/$file" 2>/dev/null || shasum -a 256 "$tmp/$file") | cut -d' ' -f1)"
	if [ "$want" != "$got" ]; then
		echo "Go download checksum mismatch; aborting." >&2
		exit 1
	fi
	tar -xzf "$tmp/$file" -C "$root" # the tarball contains a top-level "go" folder
fi
[ -z "$go" ] && go="$local_go"

# --- collector ----------------------------------------------------------------
cd "$root"
echo "Running tests..."
"$go" test ./...

# -trimpath keeps local paths (user name, module cache) out of the binary.
echo "Building agentmeter into $dest..."
mkdir -p "$dest"
"$go" build -trimpath -ldflags "-s -w" -o "$dest/agentmeter" .
install -m 755 unix/run.sh "$dest/run.sh"
install -m 755 unix/xbar/agentmeter.1m.sh "$dest/agentmeter.1m.sh"
ok "agentmeter + run.sh in $dest"

# --- front ends -----------------------------------------------------------------
setup_macos() {
	# SwiftBar asks for a plugin folder on first launch and stores it here.
	swiftbar_dir="$(defaults read com.ameba.SwiftBar PluginDirectory 2>/dev/null || true)"
	if [ -n "$swiftbar_dir" ] && [ -d "$swiftbar_dir" ]; then
		rm -f "$swiftbar_dir/agentmeter.1m.js" # previous JavaScript version of the plugin
		install -m 755 unix/xbar/agentmeter.1m.sh "$swiftbar_dir/"
		ok "SwiftBar plugin in $swiftbar_dir"
	elif [ -d /Applications/SwiftBar.app ]; then
		todo "SwiftBar: open it once and pick a plugin folder, then re-run"
	fi

	ubersicht_dir="$HOME/Library/Application Support/Übersicht/widgets"
	if [ -d "$ubersicht_dir" ]; then
		mkdir -p "$ubersicht_dir/agentmeter.widget"
		cp unix/ubersicht/agentmeter.widget/index.jsx "$ubersicht_dir/agentmeter.widget/"
		ok "Übersicht widget in $ubersicht_dir/agentmeter.widget"
	elif [ -d "/Applications/Übersicht.app" ]; then
		todo "Übersicht: launch it once, then re-run"
	fi

	if [ -z "$swiftbar_dir" ] && [ ! -d "$ubersicht_dir" ]; then
		todo "No front end found: brew install --cask swiftbar ubersicht, open them once, re-run"
	fi
}

setup_linux() {
	desktop="$(echo "${XDG_CURRENT_DESKTOP:-}:${DESKTOP_SESSION:-}" | tr '[:upper:]' '[:lower:]')"
	echo "Desktop: ${XDG_CURRENT_DESKTOP:-unknown} (${XDG_SESSION_TYPE:-unknown session})"
	bar=""

	case "$desktop" in
	*gnome* | *ubuntu* | *unity* | *pop*)
		bar=argos
		mkdir -p "$HOME/.config/argos"
		install -m 755 unix/xbar/agentmeter.1m.sh "$HOME/.config/argos/"
		ok "Argos plugin in ~/.config/argos"
		if ! gnome-extensions info argos@pew.worldwidemann.com >/dev/null 2>&1; then
			todo "GNOME: install the Argos extension (https://extensions.gnome.org/extension/1176/argos/); the plugin shows up as soon as it is enabled"
		fi
		;;
	*kde* | *plasma*)
		bar=kargos
		ok "Kargos plugin in $dest/agentmeter.1m.sh"
		if [ ! -d "$HOME/.local/share/plasma/plasmoids/org.kde.kargos" ] && [ ! -d /usr/share/plasma/plasmoids/org.kde.kargos ]; then
			todo "KDE: install the Kargos widget (Add Widgets > Get New Widgets > \"kargos\")"
		fi
		todo "KDE: add Kargos to a panel, set its command to $dest/agentmeter.1m.sh and the interval to 60 s"
		;;
	esac

	if command -v waybar >/dev/null 2>&1; then
		bar="${bar:+$bar+}waybar"
		todo "Waybar: add the \"custom/agentmeter\" module from unix/waybar/README.md to your config, then killall -SIGUSR2 waybar"
	else
		case "$desktop" in
		*hyprland* | *sway* | *niri* | *river* | *wayfire* | *labwc*)
			todo "${XDG_CURRENT_DESKTOP}: install Waybar and re-run for a bar module" ;;
		esac
	fi

	if command -v conky >/dev/null 2>&1; then
		mkdir -p "$HOME/.config/conky"
		cp unix/conky/agentmeter.conf "$HOME/.config/conky/agentmeter.conf"
		if [ -z "$bar" ]; then
			# No bar front end fits this desktop: make the Conky panel the front end.
			mkdir -p "$HOME/.config/autostart"
			cat >"$HOME/.config/autostart/agentmeter-conky.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=AgentMeter (Conky)
Exec=conky -d -p 5 -c $HOME/.config/conky/agentmeter.conf
X-GNOME-Autostart-enabled=true
EOF
			pkill -f "conky.*agentmeter.conf" 2>/dev/null || true
			conky -d -c "$HOME/.config/conky/agentmeter.conf" >/dev/null 2>&1 || true
			ok "Conky panel started and added to autostart"
		else
			ok "Conky config in ~/.config/conky/agentmeter.conf (optional: conky -c ~/.config/conky/agentmeter.conf)"
		fi
	elif [ -z "$bar" ]; then
		todo "No bar front end for this desktop: install conky (apt/dnf/pacman install conky) and re-run for a desktop panel"
	fi

	# Gemini (Antigravity) keeps its token in the Secret Service.
	if ! command -v secret-tool >/dev/null 2>&1; then
		todo "Gemini needs secret-tool: apt install libsecret-tools / dnf install libsecret / pacman -S libsecret"
	fi
}

case "$os" in
Darwin) setup_macos ;;
Linux) setup_linux ;;
*) todo "Unsupported OS $os: agentmeter is built, but no front end was set up" ;;
esac

printf "\nDone:%b\n" "$done_msgs"
[ -n "$todo_msgs" ] && printf "\nStill to do:%b\n" "$todo_msgs"
printf "\nData, state and log live in %s. Extra flags (e.g. -providers claude,codex) go in %s/flags.\n" "$dest" "$dest"
