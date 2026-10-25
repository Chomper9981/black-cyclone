#!/bin/sh
# Builds build/bin/BlackCyclone for Linux. Extra arguments go to `wails build`.
# Distros from 2024 on (Ubuntu 24.04, Debian 13, Fedora 40) ship only
# WebKitGTK 4.1, which Wails selects with the webkit2_41 build tag.
# Usage: linux/build.sh [-clean] [-debug]
set -eu
cd "$(dirname "$0")/.."

tags=
if pkg-config --exists webkit2gtk-4.1; then
    tags=webkit2_41
elif ! pkg-config --exists webkit2gtk-4.0; then
    echo "WebKitGTK not found: install libwebkit2gtk-4.1-dev (see linux/README.md)" >&2
    exit 1
fi

wails build ${tags:+-tags "$tags"} "$@"
