#!/bin/sh
# Installs Black Cyclone system-wide:
#   /opt/blackcyclone/BlackCyclone   the app, with CAP_NET_ADMIN + CAP_NET_RAW
#   /opt/blackcyclone/bin/           sing-box and geoip-vn.srs
#   /opt/blackcyclone/licenses/      license texts (with LICENSE, THIRD_PARTY_NOTICES.md)
#   /usr/local/bin/blackcyclone      launcher symlink
#   menu entry + icon under /usr/share
# The app hands its two capabilities to the sing-box it starts, so everything
# under /opt/blackcyclone stays root-owned: a user who could replace sing-box
# there could run any program with them.
# Usage: sudo linux/install.sh            (after linux/build.sh and linux/fetch-bins.sh)
#        sudo linux/install.sh --uninstall
set -eu

PREFIX=/opt/blackcyclone
DESKTOP=/usr/share/applications/blackcyclone.desktop
ICON=/usr/share/icons/hicolor/256x256/apps/blackcyclone.png
LINK=/usr/local/bin/blackcyclone
ROOT=$(cd "$(dirname "$0")/.." && pwd)

if [ "$(id -u)" -ne 0 ]; then
    echo "Run as root: sudo $0 $*" >&2
    exit 1
fi

refresh() {
    gtk-update-icon-cache -q -t /usr/share/icons/hicolor 2>/dev/null || true
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
}

if [ "${1:-}" = "--uninstall" ]; then
    pkill -x BlackCyclone 2>/dev/null || true
    rm -rf "$PREFIX"
    rm -f "$DESKTOP" "$ICON" "$LINK"
    refresh
    echo "Removed. Each user's settings stay in ~/.config/BlackCyclone; the logon entry, if enabled, is ~/.config/autostart/blackcyclone.desktop."
    exit 0
fi

if ! command -v setcap >/dev/null 2>&1; then
    echo "setcap not found: install libcap2-bin (Debian/Ubuntu) or libcap (Fedora/Arch)" >&2
    exit 1
fi
APP="$ROOT/build/bin/BlackCyclone"
if [ ! -x "$APP" ]; then
    echo "$APP not found: run linux/build.sh first" >&2
    exit 1
fi
if [ ! -x "$ROOT/bin/sing-box" ]; then
    echo "$ROOT/bin/sing-box not found: run linux/fetch-bins.sh first" >&2
    exit 1
fi

# A running copy would keep the old binary and its TUN device.
pkill -x BlackCyclone 2>/dev/null || true

install -d -m 0755 -o root -g root "$PREFIX" "$PREFIX/bin"
install -m 0755 -o root -g root "$APP" "$PREFIX/BlackCyclone"
install -m 0755 -o root -g root "$ROOT/bin/sing-box" "$PREFIX/bin/sing-box"
if [ -f "$ROOT/bin/geoip-vn.srs" ]; then
    install -m 0644 -o root -g root "$ROOT/bin/geoip-vn.srs" "$PREFIX/bin/geoip-vn.srs"
fi
setcap cap_net_admin,cap_net_raw+ep "$PREFIX/BlackCyclone"
# License texts that must travel with sing-box (GPL-3.0) and the Go libraries.
install -m 0644 -o root -g root "$ROOT/LICENSE" "$PREFIX/LICENSE"
install -m 0644 -o root -g root "$ROOT/THIRD_PARTY_NOTICES.md" "$PREFIX/THIRD_PARTY_NOTICES.md"
install -d -m 0755 -o root -g root "$PREFIX/licenses"
install -m 0644 -o root -g root "$ROOT/licenses/GPL-3.0.txt" "$ROOT/licenses/go-desktop.txt" "$PREFIX/licenses/"

install -D -m 0644 "$ROOT/linux/blackcyclone.png" "$ICON"
install -D -m 0644 "$ROOT/linux/blackcyclone.desktop" "$DESKTOP"
ln -sf "$PREFIX/BlackCyclone" "$LINK"
refresh

echo "Installed to $PREFIX. Open \"Black Cyclone\" from the app menu, or run: blackcyclone"
