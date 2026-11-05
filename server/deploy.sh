#!/usr/bin/env bash
# Dựng (hoặc cập nhật) server Black Cyclone lên một VPS, chạy từ máy Linux / macOS.
# Chép server/bcserver.sh lên VPS rồi chạy "bcserver install"; in ra link cho app.
#
# Dùng:   server/deploy.sh [-p cổng_ssh] [-i khoá_ssh] user@vps [tuỳ chọn install...]
# Ví dụ:  server/deploy.sh root@203.0.113.10
#         server/deploy.sh -i ~/.ssh/id_ed25519 ubuntu@203.0.113.10 --port 8443 --sni www.apple.com
#
# Chạy lại bất cứ lúc nào để cập nhật: khoá và người dùng trên VPS được giữ nguyên.
set -euo pipefail

ssh_opts=() scp_opts=()
while (($#)); do
    case $1 in
    -p) ssh_opts+=(-p "$2") scp_opts+=(-P "$2") && shift 2 ;;
    -i) ssh_opts+=(-i "$2") scp_opts+=(-i "$2") && shift 2 ;;
    *) break ;;
    esac
done
if (($# < 1)); then
    sed -n '5,7p' "$0" | sed 's/^# \{0,1\}//' >&2
    exit 1
fi
target=$1
shift

dir=$(cd "$(dirname "$0")" && pwd)
user=${target%%@*}
[ "$user" = "$target" ] && user=$(id -un)
sudo=
[ "$user" = root ] || sudo='sudo '

# scp needs brackets around an IPv6 address.
host=${target#*@}
scp_target=$target
[[ $host == *:* ]] && scp_target="$user@[$host]"

echo "==> Chép bcserver.sh lên $host"
scp ${scp_opts[@]+"${scp_opts[@]}"} "$dir/bcserver.sh" "$scp_target:bcserver.sh"

echo "==> Cài đặt trên VPS"
remote="${sudo}bash ~/bcserver.sh install"
for a in "$@"; do
    remote+=" $(printf '%q' "$a")"
done
remote+='; rc=$?; rm -f ~/bcserver.sh; exit $rc'
ssh -t ${ssh_opts[@]+"${ssh_opts[@]}"} "$target" "$remote"

echo
echo "Xong. Copy link vless:// ở trên, mở Black Cyclone > Server riêng > dán link."
echo "Quản lý về sau: ssh $target, rồi chạy ${sudo}bcserver help"
