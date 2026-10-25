package tunnel

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"

	"blackcyclone-vpn-mobile/core/warp"
)

// Tunnel is a running WireGuard device.
type Tunnel struct {
	dev *device.Device
}

// Stats is a snapshot of the peer counters.
type Stats struct {
	RxBytes       int64
	TxBytes       int64
	LastHandshake time.Time // zero if no handshake yet
}

// Start brings a WireGuard device up on the TUN fd, owned by the device from here on.
// logf receives wireguard-go's log lines.
func Start(fd int, a *warp.Account, logf func(string)) (*Tunnel, error) {
	uapi, reserved, err := uapiConfig(a)
	if err != nil {
		os.NewFile(uintptr(fd), "tun").Close()
		return nil, err
	}
	tunDev, err := openTUN(fd)
	if err != nil {
		return nil, fmt.Errorf("mở TUN: %w", err)
	}
	logger := &device.Logger{
		Verbosef: func(format string, args ...any) { logf(fmt.Sprintf(format, args...)) },
		Errorf:   func(format string, args ...any) { logf("LỖI: " + fmt.Sprintf(format, args...)) },
	}
	dev := device.NewDevice(tunDev, newReservedBind(conn.NewStdNetBind(), reserved), logger)
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("cấu hình WireGuard: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("bật WireGuard: %w", err)
	}
	return &Tunnel{dev: dev}, nil
}

// Close shuts the device down and closes the TUN fd.
func (t *Tunnel) Close() { t.dev.Close() }

// Rebind reopens the UDP sockets, e.g. after switching between Wi-Fi and mobile data.
func (t *Tunnel) Rebind() error { return t.dev.BindUpdate() }

func (t *Tunnel) Stats() (Stats, error) {
	cfg, err := t.dev.IpcGet()
	if err != nil {
		return Stats{}, err
	}
	return parseStats(cfg), nil
}

func parseStats(cfg string) Stats {
	var s Stats
	var sec, nsec int64
	sc := bufio.NewScanner(strings.NewReader(cfg))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		switch k {
		case "rx_bytes":
			s.RxBytes += n
		case "tx_bytes":
			s.TxBytes += n
		case "last_handshake_time_sec":
			sec = n
		case "last_handshake_time_nsec":
			nsec = n
		}
	}
	if sec != 0 || nsec != 0 {
		s.LastHandshake = time.Unix(sec, nsec)
	}
	return s
}

func uapiConfig(a *warp.Account) (string, [3]byte, error) {
	var reserved [3]byte
	if a == nil || a.PrivateKey == "" {
		return "", reserved, fmt.Errorf("chưa đăng ký WARP")
	}
	priv, err := keyHex(a.PrivateKey)
	if err != nil {
		return "", reserved, fmt.Errorf("warp private key không hợp lệ")
	}
	peer, err := keyHex(a.PeerPublicKey)
	if err != nil {
		return "", reserved, fmt.Errorf("warp peer public key không hợp lệ")
	}
	if a.Endpoint == "" || a.EndpointPort <= 0 || a.EndpointPort > 65535 {
		return "", reserved, fmt.Errorf("endpoint WARP không hợp lệ")
	}
	ep := a.Endpoint
	if ip := net.ParseIP(ep); ip == nil {
		// wireguard-go only takes IP endpoints.
		ips, err := net.LookupIP(ep)
		if err != nil || len(ips) == 0 {
			return "", reserved, fmt.Errorf("phân giải endpoint %s: %v", ep, err)
		}
		ep = ips[0].String()
	}
	for i := 0; i < 3 && i < len(a.Reserved); i++ {
		reserved[i] = byte(a.Reserved[i])
	}

	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\n", priv)
	b.WriteString("replace_peers=true\n")
	fmt.Fprintf(&b, "public_key=%s\n", peer)
	fmt.Fprintf(&b, "endpoint=%s\n", net.JoinHostPort(ep, strconv.Itoa(a.EndpointPort)))
	b.WriteString("persistent_keepalive_interval=25\n")
	b.WriteString("replace_allowed_ips=true\n")
	b.WriteString("allowed_ip=0.0.0.0/0\n")
	b.WriteString("allowed_ip=::/0\n")
	return b.String(), reserved, nil
}

func keyHex(b64 string) (string, error) {
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(k) != 32 {
		return "", fmt.Errorf("bad key")
	}
	return hex.EncodeToString(k), nil
}
