// Package singbox builds a sing-box (1.12+ format) configuration from the app settings.
package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"redcyclone-vpn/internal/settings"
	"redcyclone-vpn/internal/share"
)

// Build returns the sing-box JSON config for the selected mode.
func Build(s settings.Settings) ([]byte, error) {
	doh, err := dohServer(s.DoHServer)
	if err != nil {
		return nil, err
	}

	// "dns-direct": DoH straight from this machine. Used in bypass mode, and to resolve
	// the proxy server's own hostname in the other modes.
	direct := clone(doh)
	direct["tag"] = "dns-direct"
	servers := []any{
		map[string]any{"type": "local", "tag": "dns-local"},
		direct,
	}
	finalDNS := "dns-direct"
	finalOut := "direct"

	outbounds := []any{map[string]any{"type": "direct", "tag": "direct"}}
	var endpoints []any

	switch s.Mode {
	case settings.ModeBypass:
	case settings.ModeWarp:
		if s.Warp == nil {
			return nil, fmt.Errorf("chưa đăng ký WARP")
		}
		endpoints = append(endpoints, warpEndpoint(s.Warp))
		finalOut = "proxy"
	case settings.ModeCustom:
		ob, err := share.ParseOutbound(s.CustomLink, "proxy")
		if err != nil {
			return nil, fmt.Errorf("link server: %w", err)
		}
		outbounds = append(outbounds, ob)
		finalOut = "proxy"
	default:
		return nil, fmt.Errorf("mode không hợp lệ: %q", s.Mode)
	}

	if finalOut == "proxy" {
		// Resolve through the tunnel so the ISP never sees the queries.
		remote := clone(doh)
		remote["tag"] = "dns-remote"
		remote["detour"] = "proxy"
		servers = append(servers, remote)
		finalDNS = "dns-remote"
	}

	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
		// Our own helpers must never loop back into the tunnel.
		map[string]any{"process_name": []string{"sing-box.exe", "goodbyedpi.exe"}, "outbound": "direct"},
	}
	if s.BypassLAN {
		rules = append(rules, map[string]any{"ip_is_private": true, "outbound": "direct"})
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": map[string]any{
			"servers":  servers,
			"final":    finalDNS,
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": "RedCyclone",
			"address":        []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			"mtu":            9000,
			"auto_route":     true,
			"strict_route":   true,
			"stack":          "mixed",
		}},
		"outbounds": outbounds,
		"route": map[string]any{
			"rules":                   rules,
			"final":                   finalOut,
			"auto_detect_interface":   true,
			"default_domain_resolver": "dns-direct",
		},
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// dohServer converts "https://host[:port]/path" into a sing-box https DNS server.
func dohServer(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, fmt.Errorf("DoH server phải có dạng https://host/dns-query, nhận %q", raw)
	}
	srv := map[string]any{"type": "https", "server": u.Hostname()}
	if u.Path != "" && u.Path != "/dns-query" {
		srv["path"] = u.Path
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("DoH port không hợp lệ: %q", p)
		}
		srv["server_port"] = port
	}
	if net.ParseIP(u.Hostname()) == nil {
		// Bootstrapping a DoH hostname needs a plain resolver.
		srv["domain_resolver"] = "dns-local"
	}
	return srv, nil
}

func warpEndpoint(w *settings.Warp) map[string]any {
	return map[string]any{
		"type":        "wireguard",
		"tag":         "proxy",
		"mtu":         1280,
		"address":     w.Addresses,
		"private_key": w.PrivateKey,
		"peers": []any{map[string]any{
			"address":     w.Endpoint,
			"port":        w.EndpointPort,
			"public_key":  w.PeerPublicKey,
			"allowed_ips": []string{"0.0.0.0/0", "::/0"},
			"reserved":    w.Reserved,
		}},
	}
}

func clone(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
