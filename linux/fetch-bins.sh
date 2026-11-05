#!/bin/sh
# Downloads the runtime binary (sing-box) for Linux into <project>/bin.
# Same pinned version as windows/fetch-bins.ps1, verified by SHA-256: sing-box
# changes its config format between releases, so bump both together with
# internal/singbox and its tests.
# Usage: linux/fetch-bins.sh
set -eu

version=1.14.3
case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 sha256=e2bdf179c15a3652955dc44867e33ca0965c9c9b91b34267dcc5a5d639a5feee ;;
    aarch64 | arm64) arch=arm64 sha256=29dae24d76bea3bc62d07dd3e60cfc12003de7debbd6b768df9be9c68360174e ;;
    *)
        echo "unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac
url="https://github.com/SagerNet/sing-box/releases/download/v$version/sing-box-$version-linux-$arch.tar.gz"
# Vietnamese IP ranges for "Web Việt Nam đi thẳng"; refreshed weekly upstream, so not pinned.
# Optional: without it sing-box downloads the same file at connect time.
geoip_url=https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-vn.srs

root=$(cd "$(dirname "$0")/.." && pwd)
bin="$root/bin"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$bin"

echo "Downloading sing-box: $url"
curl -fL --retry 3 -o "$tmp/sing-box.tar.gz" "$url"
if ! echo "$sha256  $tmp/sing-box.tar.gz" | sha256sum -c --status -; then
    echo "sing-box: SHA-256 mismatch (got $(sha256sum "$tmp/sing-box.tar.gz" | cut -d' ' -f1), want $sha256)" >&2
    exit 1
fi
tar -xzf "$tmp/sing-box.tar.gz" -C "$tmp"
install -m 0755 "$tmp/sing-box-$version-linux-$arch/sing-box" "$bin/sing-box"

if ! curl -fsSL --retry 3 -o "$bin/geoip-vn.srs" "$geoip_url"; then
    echo "warning: geoip-vn.srs download failed (optional)" >&2
fi

ls -l "$bin"
