#!/bin/sh
# Builds dist/blackcyclone_<version>_<arch>.deb: the app, sing-box and
# geoip-vn.srs under /opt/blackcyclone (same layout as linux/install.sh), plus
# the menu entry, icon, /usr/bin/blackcyclone and the license texts.
# The postinst grants CAP_NET_ADMIN + CAP_NET_RAW, like install.sh.
#
# Needs Go, a C compiler, pkg-config and the GTK 3 / WebKitGTK 4.1 dev packages
# (see linux/README.md). The frontend (frontend/dist) must already be built:
# `wails build` or `cd frontend && npm run build`. linux/docker-deb.ps1 runs
# this inside an Ubuntu 22.04 container for amd64 and arm64.
# Usage: linux/package-deb.sh
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

version=$(sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
arch=$(dpkg --print-architecture)
[ -n "$version" ] || { echo "productVersion not found in wails.json" >&2; exit 1; }
[ -f frontend/dist/index.html ] || { echo "frontend/dist missing: build the frontend first" >&2; exit 1; }
if ! pkg-config --exists webkit2gtk-4.1; then
    echo "WebKitGTK 4.1 not found: install libwebkit2gtk-4.1-dev" >&2
    exit 1
fi

echo "Building Black Cyclone $version ($arch)"
mkdir -p build/bin
CGO_ENABLED=1 go build -tags desktop,production,webkit2_41 -trimpath -ldflags "-s -w" -o build/bin/BlackCyclone .
[ -x bin/sing-box ] || linux/fetch-bins.sh

pkg=$(mktemp -d)
trap 'rm -rf "$pkg"' EXIT
opt="$pkg/opt/blackcyclone"
install -d "$opt/bin" "$opt/licenses" "$pkg/usr/bin" "$pkg/DEBIAN" \
    "$pkg/usr/share/applications" "$pkg/usr/share/icons/hicolor/256x256/apps"
install -m 0755 build/bin/BlackCyclone "$opt/BlackCyclone"
install -m 0755 bin/sing-box "$opt/bin/sing-box"
[ -f bin/geoip-vn.srs ] && install -m 0644 bin/geoip-vn.srs "$opt/bin/geoip-vn.srs"
install -m 0644 LICENSE THIRD_PARTY_NOTICES.md "$opt/"
install -m 0644 licenses/GPL-3.0.txt licenses/go-desktop.txt "$opt/licenses/"
install -m 0644 linux/blackcyclone.desktop "$pkg/usr/share/applications/blackcyclone.desktop"
install -m 0644 linux/blackcyclone.png "$pkg/usr/share/icons/hicolor/256x256/apps/blackcyclone.png"
ln -s /opt/blackcyclone/BlackCyclone "$pkg/usr/bin/blackcyclone"

size=$(du -sk "$pkg" | cut -f1)
cat > "$pkg/DEBIAN/control" <<EOF
Package: blackcyclone
Version: $version
Architecture: $arch
Maintainer: Red Cyclone <126317880+Chomper9981@users.noreply.github.com>
Installed-Size: $size
Depends: libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0, libcap2-bin
Section: net
Priority: optional
Homepage: https://github.com/Chomper9981/black-cyclone
Description: VPN vượt chặn nhà mạng: Cloudflare WARP hoặc server riêng
 Black Cyclone chạy sing-box làm lõi TUN, đưa toàn bộ lưu lượng qua
 Cloudflare WARP hoặc server riêng của bạn (VLESS + Reality…).
EOF

cat > "$pkg/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
# The app hands these to the sing-box it starts (see linux/install.sh).
setcap cap_net_admin,cap_net_raw+ep /opt/blackcyclone/BlackCyclone
gtk-update-icon-cache -q -t /usr/share/icons/hicolor 2>/dev/null || true
update-desktop-database -q /usr/share/applications 2>/dev/null || true
EOF
cat > "$pkg/DEBIAN/prerm" <<'EOF'
#!/bin/sh
# A running copy keeps the old binary and its TUN device.
pkill -x BlackCyclone 2>/dev/null || true
pkill -f '^/opt/blackcyclone/bin/sing-box' 2>/dev/null || true
exit 0
EOF
chmod 0755 "$pkg/DEBIAN/postinst" "$pkg/DEBIAN/prerm"

mkdir -p dist
out="dist/blackcyclone_${version}_${arch}.deb"
dpkg-deb --root-owner-group -Zxz --build "$pkg" "$out"
echo "Built $out"
