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

# Language of the messages: BC_LANG=vi|en, else the shell's locale (Vietnamese
# locales get Vietnamese, everything else English). bcserver on the VPS gets the same.
case ${BC_LANG:-${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}} in
vi | vi_* | vi-*) BC_LANG=vi ;;
*) BC_LANG=en ;;
esac

# L "tiếng Việt" "English": the message in the chosen language.
L() { if [ "$BC_LANG" = vi ]; then printf '%s' "$1"; else printf '%s' "$2"; fi; }

usage() {
    if [ "$BC_LANG" = vi ]; then
        cat <<'EOF'
Dùng:   server/deploy.sh [-p cổng_ssh] [-i khoá_ssh] user@vps [tuỳ chọn install...]
Ví dụ:  server/deploy.sh root@203.0.113.10
        server/deploy.sh -i ~/.ssh/id_ed25519 ubuntu@203.0.113.10 --port 8443 --sni www.apple.com
EOF
    else
        cat <<'EOF'
Usage:    server/deploy.sh [-p ssh_port] [-i ssh_key] user@vps [install options...]
Example:  server/deploy.sh root@203.0.113.10
          server/deploy.sh -i ~/.ssh/id_ed25519 ubuntu@203.0.113.10 --port 8443 --sni www.apple.com
EOF
    fi
}

ssh_opts=() scp_opts=()
while (($#)); do
    case $1 in
    -p) ssh_opts+=(-p "$2") scp_opts+=(-P "$2") && shift 2 ;;
    -i) ssh_opts+=(-i "$2") scp_opts+=(-i "$2") && shift 2 ;;
    *) break ;;
    esac
done
if (($# < 1)); then
    usage >&2
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

echo "==> $(L "Chép bcserver.sh lên $host" "Copying bcserver.sh to $host")"
scp ${scp_opts[@]+"${scp_opts[@]}"} "$dir/bcserver.sh" "$scp_target:bcserver.sh"

echo "==> $(L "Cài đặt trên VPS" "Installing on the VPS")"
# BC_LANG goes after sudo (sudo takes VAR=value before the command) so that
# bcserver answers in the same language.
remote="${sudo}BC_LANG=$BC_LANG bash ~/bcserver.sh install"
for a in "$@"; do
    remote+=" $(printf '%q' "$a")"
done
remote+='; rc=$?; rm -f ~/bcserver.sh; exit $rc'
ssh -t ${ssh_opts[@]+"${ssh_opts[@]}"} "$target" "$remote"

echo
echo "$(L "Xong. Copy link vless:// ở trên, mở Black Cyclone > Server riêng > dán link." "Done. Copy the vless:// link above, open Black Cyclone > Own server > paste the link.")"
echo "$(L "Quản lý về sau: ssh $target, rồi chạy ${sudo}bcserver help" "To manage it later: ssh $target, then run ${sudo}bcserver help")"
