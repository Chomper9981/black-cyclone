#!/usr/bin/env bash
# bcserver: dựng và quản lý server riêng cho Black Cyclone trên một VPS Linux trống.
#
# Server chạy sing-box (cùng phiên bản với app) với VLESS + Reality: không cần
# tên miền hay chứng chỉ. Với nhà mạng, traffic trông như truy cập HTTPS tới một
# trang lớn (mặc định www.microsoft.com), nên khó bị nhận ra để chặn.
#
# Lần đầu (hoặc chạy lại để cập nhật / đổi cấu hình, khoá và người dùng được giữ):
#   sudo bash bcserver.sh install [--port 443] [--sni www.microsoft.com] [--host IP|tên miền] [--name tên]
# Sau khi cài, script nằm ở /usr/local/bin/bcserver: chạy `bcserver help` để xem lệnh.
#
# Cần: Linux có systemd (Ubuntu, Debian, Rocky, Alma…), x86_64 hoặc arm64, quyền root.
set -euo pipefail

# Cùng phiên bản và SHA-256 với linux/fetch-bins.sh: cập nhật cả hai cùng lúc.
SB_VERSION=1.14.3
SB_SHA256_AMD64=e2bdf179c15a3652955dc44867e33ca0965c9c9b91b34267dcc5a5d639a5feee
SB_SHA256_ARM64=29dae24d76bea3bc62d07dd3e60cfc12003de7debbd6b768df9be9c68360174e

ETC=${BC_ETC:-/etc/blackcyclone}
OPT=/opt/blackcyclone
SB=${BC_SINGBOX:-$OPT/sing-box}
SELF=/usr/local/bin/bcserver
SERVICE=blackcyclone
UNIT=/etc/systemd/system/$SERVICE.service
SYSCTL=/etc/sysctl.d/99-blackcyclone.conf
SVC_USER=blackcyclone

STATE=$ETC/server.conf # PORT, SNI, HOST, khoá Reality
USERS=$ETC/users       # mỗi dòng: "tên uuid"
CONFIG=$ETC/config.json

DEFAULT_PORT=443
DEFAULT_SNI=www.microsoft.com
DEFAULT_USER=default

PORT= SNI= HOST= PRIVATE_KEY= PUBLIC_KEY= SHORT_ID=

die() { echo "Lỗi: $*" >&2; exit 1; }
info() { echo "==> $*"; }
warn() { echo "Cảnh báo: $*" >&2; }

need_root() { [ "$(id -u)" -eq 0 ] || die "cần quyền root: chạy lại với sudo"; }

valid_name() { [[ $1 =~ ^[A-Za-z0-9._-]{1,32}$ ]]; }
valid_port() { [[ $1 =~ ^[0-9]{1,5}$ ]] && (($1 >= 1 && $1 <= 65535)); }
valid_domain() { [[ $1 =~ ^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$ && $1 == *.* ]]; }
valid_host() { [[ $1 =~ ^[A-Za-z0-9.:-]{1,253}$ ]]; }
valid_key() { [[ $1 =~ ^[A-Za-z0-9_-]{40,50}$ ]]; }

# ---------------------------------------------------------------- state

# read_state loads server.conf without executing it.
read_state() {
    [ -f "$STATE" ] || die "server chưa được cài: chạy bcserver install"
    local k v
    while IFS='=' read -r k v; do
        case $k in
        PORT | SNI | HOST | PRIVATE_KEY | PUBLIC_KEY | SHORT_ID) printf -v "$k" '%s' "$v" ;;
        esac
    done <"$STATE"
    valid_port "$PORT" && valid_domain "$SNI" && valid_host "$HOST" && valid_key "$PRIVATE_KEY" &&
        valid_key "$PUBLIC_KEY" && [[ $SHORT_ID =~ ^[0-9a-f]{16}$ ]] || die "$STATE bị hỏng"
}

write_state() {
    (
        umask 077
        printf 'PORT=%s\nSNI=%s\nHOST=%s\nPRIVATE_KEY=%s\nPUBLIC_KEY=%s\nSHORT_ID=%s\n' \
            "$PORT" "$SNI" "$HOST" "$PRIVATE_KEY" "$PUBLIC_KEY" "$SHORT_ID" >"$STATE.tmp"
    )
    mv "$STATE.tmp" "$STATE"
}

# gen_keys creates the Reality key pair and short id with sing-box itself.
gen_keys() {
    local out
    out=$("$SB" generate reality-keypair)
    PRIVATE_KEY=$(awk '/PrivateKey/ {print $2}' <<<"$out")
    PUBLIC_KEY=$(awk '/PublicKey/ {print $2}' <<<"$out")
    SHORT_ID=$("$SB" generate rand --hex 8)
    valid_key "$PRIVATE_KEY" && valid_key "$PUBLIC_KEY" || die "không tạo được khoá Reality: $out"
}

user_uuid() { awk -v n="$1" '$1 == n {print $2; exit}' "$USERS" 2>/dev/null; }

add_user() {
    local name=$1 uuid
    uuid=$("$SB" generate uuid)
    (
        umask 077
        printf '%s %s\n' "$name" "$uuid" >>"$USERS"
    )
}

user_count() { [ -f "$USERS" ] && awk 'NF == 2' "$USERS" | wc -l || echo 0; }

# ---------------------------------------------------------------- config / links

users_json() {
    local first=1 name uuid
    while read -r name uuid; do
        [ -n "$uuid" ] || continue
        ((first)) || printf ',\n'
        first=0
        printf '        {"name": "%s", "uuid": "%s", "flow": "xtls-rprx-vision"}' "$name" "$uuid"
    done <"$USERS"
}

render_config() {
    local listen=0.0.0.0
    [ -e /proc/net/if_inet6 ] && listen=::
    cat <<EOF
{
  "log": {"level": "warn", "timestamp": true},
  "dns": {"servers": [{"type": "local", "tag": "local"}]},
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-in",
      "listen": "$listen",
      "listen_port": $PORT,
      "users": [
$(users_json)
      ],
      "tls": {
        "enabled": true,
        "server_name": "$SNI",
        "reality": {
          "enabled": true,
          "handshake": {"server": "$SNI", "server_port": 443},
          "private_key": "$PRIVATE_KEY",
          "short_id": ["$SHORT_ID"]
        }
      }
    }
  ],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {
    "rules": [
      {"action": "sniff"},
      {"ip_is_private": true, "action": "reject"}
    ],
    "final": "direct",
    "default_domain_resolver": "local"
  }
}
EOF
}

# apply_config regenerates config.json, validates it and restarts the service if it runs.
apply_config() {
    local tmp=$CONFIG.tmp
    render_config >"$tmp"
    if ! "$SB" check -c "$tmp"; then
        rm -f "$tmp"
        die "cấu hình mới không hợp lệ, giữ nguyên cấu hình cũ"
    fi
    if id "$SVC_USER" >/dev/null 2>&1; then
        chown "root:$SVC_USER" "$tmp"
        chmod 0640 "$tmp"
    fi
    mv "$tmp" "$CONFIG"
    if command -v systemctl >/dev/null && systemctl is-active --quiet "$SERVICE"; then
        systemctl restart "$SERVICE"
    fi
}

# link_for prints the share link of one user, in the format the app parses.
link_for() {
    local name=$1 uuid=$2 h=$HOST
    [[ $h == *:* ]] && h="[$h]"
    printf 'vless://%s@%s:%s?encryption=none&flow=xtls-rprx-vision&security=reality&sni=%s&fp=chrome&pbk=%s&sid=%s&type=tcp#%s-%s\n' \
        "$uuid" "$h" "$PORT" "$SNI" "$PUBLIC_KEY" "$SHORT_ID" "$HOST" "$name"
}

print_links() {
    local only=${1:-} name uuid found=0
    while read -r name uuid; do
        [ -n "$uuid" ] || continue
        [ -z "$only" ] || [ "$name" = "$only" ] || continue
        found=1
        echo "[$name]"
        link_for "$name" "$uuid"
        echo
    done <"$USERS"
    ((found)) || die "không có người dùng ${only:-nào}"
}

# ---------------------------------------------------------------- install helpers

ensure_deps() {
    local missing=() c
    for c in curl tar sha256sum awk; do
        command -v "$c" >/dev/null || missing+=("$c")
    done
    ((${#missing[@]} == 0)) && return
    info "Cài thêm: ${missing[*]}"
    if command -v apt-get >/dev/null; then
        apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl tar coreutils gawk
    elif command -v dnf >/dev/null; then
        dnf install -y -q curl tar coreutils gawk
    elif command -v yum >/dev/null; then
        yum install -y -q curl tar coreutils gawk
    else
        die "thiếu ${missing[*]} và không biết cách cài trên hệ điều hành này"
    fi
}

install_singbox() {
    local arch sum
    case $(uname -m) in
    x86_64 | amd64) arch=amd64 sum=$SB_SHA256_AMD64 ;;
    aarch64 | arm64) arch=arm64 sum=$SB_SHA256_ARM64 ;;
    *) die "kiến trúc $(uname -m) chưa được hỗ trợ (cần x86_64 hoặc arm64)" ;;
    esac
    if [ -x "$SB" ] && "$SB" version 2>/dev/null | head -n1 | grep -q "version $SB_VERSION\$"; then
        info "sing-box $SB_VERSION đã có sẵn"
        return
    fi
    local url="https://github.com/SagerNet/sing-box/releases/download/v$SB_VERSION/sing-box-$SB_VERSION-linux-$arch.tar.gz"
    local tmp
    tmp=$(mktemp -d)
    info "Tải sing-box $SB_VERSION ($arch)"
    if ! curl -fsSL --retry 3 -o "$tmp/sb.tar.gz" "$url"; then
        rm -rf "$tmp"
        die "không tải được $url"
    fi
    if ! echo "$sum  $tmp/sb.tar.gz" | sha256sum -c --status -; then
        rm -rf "$tmp"
        die "sing-box: sai SHA-256, file tải về không đúng"
    fi
    tar -xzf "$tmp/sb.tar.gz" -C "$tmp"
    mkdir -p "$OPT"
    install -m 0755 "$tmp/sing-box-$SB_VERSION-linux-$arch/sing-box" "$OPT/sing-box.new"
    mv "$OPT/sing-box.new" "$SB"
    rm -rf "$tmp"
}

detect_host() {
    local u ip
    for u in https://api.ipify.org https://ipv4.icanhazip.com https://ifconfig.me/ip; do
        ip=$(curl -4 -fsS --max-time 5 "$u" 2>/dev/null | tr -d '[:space:]') || continue
        [[ $ip =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] && { echo "$ip"; return; }
    done
    ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") {print $(i + 1); exit}}')
    [ -n "$ip" ] && { echo "$ip"; return; }
    die "không tự tìm được IP công khai của VPS: chạy lại với --host <IP>"
}

check_port_free() {
    command -v ss >/dev/null || return 0
    local used
    used=$(ss -Hltnp "sport = :$PORT" 2>/dev/null | grep -v '"sing-box"' || true)
    [ -z "$used" ] || die "cổng $PORT đang bị chương trình khác dùng (tắt nó đi hoặc chọn --port khác):
$used"
}

create_service_user() {
    id "$SVC_USER" >/dev/null 2>&1 && return
    local nologin
    nologin=$(command -v nologin || echo /bin/false)
    useradd --system --no-create-home --home-dir /nonexistent --shell "$nologin" "$SVC_USER"
}

write_unit() {
    cat >"$UNIT" <<EOF
[Unit]
Description=Black Cyclone server (sing-box VLESS + Reality)
After=network-online.target
Wants=network-online.target

[Service]
User=$SVC_USER
Group=$SVC_USER
ExecStart=$SB run -c $CONFIG -D /var/lib/$SERVICE --disable-color
StateDirectory=$SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
}

# enable_bbr switches TCP to BBR congestion control, which keeps long-distance
# links (Việt Nam ↔ VPS nước ngoài) fast when packets are lost.
enable_bbr() {
    if ! grep -qw bbr /proc/sys/net/ipv4/tcp_available_congestion_control 2>/dev/null; then
        modprobe tcp_bbr 2>/dev/null || true
    fi
    if grep -qw bbr /proc/sys/net/ipv4/tcp_available_congestion_control 2>/dev/null; then
        printf 'net.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n' >"$SYSCTL"
        if sysctl -q -p "$SYSCTL" 2>/dev/null; then
            info "Đã bật TCP BBR"
        else
            warn "không bật được BBR (VPS dạng container?); server vẫn chạy bình thường"
        fi
    else
        warn "kernel không có BBR; server vẫn chạy bình thường"
    fi
}

open_firewall() {
    if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q '^Status: active'; then
        ufw allow "$PORT/tcp" >/dev/null && info "ufw: đã mở cổng $PORT/tcp"
    elif command -v firewall-cmd >/dev/null && systemctl is-active --quiet firewalld; then
        firewall-cmd -q --permanent --add-port="$PORT/tcp" && firewall-cmd -q --reload &&
            info "firewalld: đã mở cổng $PORT/tcp"
    elif command -v iptables >/dev/null && iptables -S INPUT 2>/dev/null | grep -qE -- '^-P INPUT DROP|-j (REJECT|DROP)'; then
        # Ví dụ ảnh Ubuntu của Oracle Cloud: iptables chặn sẵn mọi cổng trừ SSH.
        iptables -C INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null ||
            iptables -I INPUT -p tcp --dport "$PORT" -j ACCEPT
        info "iptables: đã mở cổng $PORT/tcp"
        if command -v netfilter-persistent >/dev/null; then
            netfilter-persistent save >/dev/null 2>&1 || warn "không lưu được luật iptables"
        else
            warn "luật iptables sẽ mất khi khởi động lại VPS: cài iptables-persistent để giữ"
        fi
    fi
}

start_service() {
    systemctl enable --quiet "$SERVICE"
    systemctl restart "$SERVICE"
    sleep 2
    if ! systemctl is-active --quiet "$SERVICE"; then
        journalctl -u "$SERVICE" -n 30 --no-pager >&2 || true
        die "server không khởi động được (xem log ở trên)"
    fi
}

check_sni() {
    curl -fsS -o /dev/null --max-time 8 --tlsv1.3 "https://$SNI" 2>/dev/null ||
        warn "VPS không truy cập được https://$SNI bằng TLS 1.3; Reality cần trang đó hoạt động: chọn --sni khác nếu kết nối lỗi"
}

# ---------------------------------------------------------------- commands

cmd_install() {
    need_root
    local port= sni= host= name=$DEFAULT_USER
    while (($#)); do
        case $1 in
        --port | --sni | --host | --name) (($# >= 2)) || die "$1 cần một giá trị" ;;
        *) die "tuỳ chọn không hợp lệ: $1 (xem bcserver help)" ;;
        esac
        case $1 in
        --port) port=$2 ;;
        --sni) sni=$2 ;;
        --host) host=$2 ;;
        --name) name=$2 ;;
        esac
        shift 2
    done
    [ -z "$port" ] || valid_port "$port" || die "--port phải là số 1-65535"
    [ -z "$sni" ] || valid_domain "$sni" || die "--sni phải là tên miền, ví dụ www.microsoft.com"
    [ -z "$host" ] || valid_host "$host" || die "--host phải là IP hoặc tên miền"
    valid_name "$name" || die "--name chỉ gồm chữ, số, '.', '_', '-' (tối đa 32 ký tự)"
    command -v systemctl >/dev/null || die "cần Linux có systemd"

    ensure_deps
    install_singbox
    create_service_user
    mkdir -p "$ETC"
    chown "root:$SVC_USER" "$ETC"
    chmod 0750 "$ETC"

    if [ -f "$STATE" ]; then
        read_state
        info "Đã có cấu hình, giữ nguyên khoá và người dùng"
    else
        gen_keys
        PORT=$DEFAULT_PORT SNI=$DEFAULT_SNI
    fi
    PORT=${port:-$PORT} SNI=${sni:-$SNI} HOST=${host:-$HOST}
    [ -n "$HOST" ] || HOST=$(detect_host)
    write_state
    if (($(user_count) == 0)); then
        add_user "$name"
    fi

    check_port_free
    check_sni
    apply_config
    write_unit
    enable_bbr
    open_firewall
    start_service

    local src=${BASH_SOURCE[0]}
    if [ -f "$src" ] && [ "$(readlink -f "$src")" != "$SELF" ]; then
        install -m 0755 "$src" "$SELF"
    fi

    echo
    info "Server đang chạy ở $HOST:$PORT (SNI $SNI)"
    echo "Nếu nhà cung cấp VPS có tường lửa riêng trên trang quản lý (security group…), mở cổng $PORT/tcp ở đó."
    echo "Thêm link dưới đây vào Black Cyclone: chế độ Server riêng → dán link."
    echo
    print_links
}

cmd_add() {
    need_root
    local name=${1:-}
    valid_name "$name" || die "dùng: bcserver add <tên> (chữ, số, '.', '_', '-')"
    read_state
    [ -z "$(user_uuid "$name")" ] || die "đã có người dùng $name"
    add_user "$name"
    apply_config
    info "Đã thêm $name"
    print_links "$name"
}

cmd_remove() {
    need_root
    local name=${1:-}
    [ -n "$name" ] || die "dùng: bcserver remove <tên>"
    read_state
    [ -n "$(user_uuid "$name")" ] || die "không có người dùng $name"
    (($(user_count) > 1)) || die "không xoá được người dùng cuối cùng"
    awk -v n="$name" '$1 != n' "$USERS" >"$USERS.tmp"
    chmod 0600 "$USERS.tmp"
    mv "$USERS.tmp" "$USERS"
    apply_config
    info "Đã xoá $name: link của người này không dùng được nữa"
}

cmd_list() {
    read_state
    awk 'NF == 2 {print $1}' "$USERS"
}

cmd_link() {
    read_state
    print_links "${1:-}"
}

cmd_status() {
    read_state
    echo "Địa chỉ: $HOST:$PORT   SNI: $SNI   sing-box: $("$SB" version 2>/dev/null | head -n1 | awk '{print $3}')"
    echo "Người dùng: $(user_count)"
    systemctl status "$SERVICE" --no-pager --lines 0 || true
}

cmd_logs() { journalctl -u "$SERVICE" -n 100 --no-pager "$@"; }

cmd_restart() {
    need_root
    systemctl restart "$SERVICE"
    systemctl is-active --quiet "$SERVICE" && info "Đã khởi động lại" || die "server không chạy (bcserver logs)"
}

cmd_uninstall() {
    need_root
    if [ "${1:-}" != "--yes" ]; then
        [ -t 0 ] || die "chạy bcserver uninstall --yes để xác nhận"
        local ans
        read -r -p "Gỡ server và xoá toàn bộ khoá, người dùng? Link cũ sẽ không dùng được nữa. [y/N] " ans
        [[ $ans == [yY]* ]] || die "đã huỷ"
    fi
    systemctl disable --now --quiet "$SERVICE" 2>/dev/null || true
    rm -f "$UNIT" "$SYSCTL"
    systemctl daemon-reload
    rm -rf "$OPT" "$ETC" "/var/lib/$SERVICE"
    userdel "$SVC_USER" 2>/dev/null || true
    rm -f "$SELF"
    info "Đã gỡ. Cổng đã mở trên tường lửa (nếu có) vẫn giữ nguyên."
}

usage() {
    cat <<'EOF'
bcserver: server riêng cho Black Cyclone (sing-box, VLESS + Reality)

  install [--port N] [--sni TÊN_MIỀN] [--host IP|TÊN_MIỀN] [--name TÊN]
                     cài hoặc cập nhật (chạy lại an toàn: giữ khoá và người dùng)
  add <tên>          thêm người dùng, in link của người đó
  remove <tên>       xoá người dùng (link của họ hết hiệu lực)
  list               liệt kê người dùng
  link [tên]         in link (tất cả, hoặc của một người)
  status             trạng thái server
  logs [-f]          xem log (-f: theo dõi liên tục)
  restart            khởi động lại server
  uninstall [--yes]  gỡ hoàn toàn

Mỗi thiết bị / người nên có một tên riêng: muốn chặn ai thì chỉ cần remove tên đó.
EOF
}

main() {
    local cmd=${1:-help}
    (($#)) && shift
    case $cmd in
    install) cmd_install "$@" ;;
    add) cmd_add "$@" ;;
    remove | rm) cmd_remove "$@" ;;
    list | ls) cmd_list ;;
    link | links) cmd_link "$@" ;;
    status) cmd_status ;;
    logs | log) cmd_logs "$@" ;;
    restart) cmd_restart ;;
    uninstall) cmd_uninstall "$@" ;;
    help | -h | --help) usage ;;
    *)
        usage >&2
        exit 1
        ;;
    esac
}

# Sourcing the file (tests) only defines the functions.
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
    main "$@"
fi
