// Package singbox builds a sing-box (1.12+ format) configuration from the app settings.
package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"redcyclone-vpn/internal/settings"
)

// Options carries runtime details that are not user settings.
type Options struct {
	BinDir      string // where bundled rule sets (geoip-vn.srs) live
	CacheFile   string // sing-box cache.db (rule-set cache, selected server)
	ClashAPI    string // "127.0.0.1:port" for stats / server switching; empty disables
	ClashSecret string
}

// Outbound tags used by the app (the Clash API addresses them by tag).
const (
	TagProxy = "proxy" // selector (custom) or WireGuard endpoint (warp)
	TagAuto  = "auto"  // urltest over all servers
)

// ServerTag is the outbound tag of a configured server.
func ServerTag(id string) string { return "srv-" + id }

// ParseLink turns a share link into a sing-box outbound with the given tag.
// It is a variable so the share-link parser can be swapped in tests.
var ParseLink = parseLink

const geoipVNURL = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-vn.srs"

// Build returns the sing-box JSON config for the selected mode, plus non-fatal
// warnings (e.g. servers whose links could not be parsed).
func Build(s settings.Settings, o Options) ([]byte, []string, error) {
	doh, err := dohServer(s.DoHServer)
	if err != nil {
		return nil, nil, err
	}

	// "dns-direct": DoH straight from this machine. Used for direct-routed
	// domains and to resolve the proxy server's own hostname.
	direct := clone(doh)
	direct["tag"] = "dns-direct"
	// "dns-remote": resolve through the tunnel so the ISP never sees the queries.
	remote := clone(doh)
	remote["tag"] = "dns-remote"
	remote["detour"] = TagProxy
	servers := []any{
		map[string]any{"type": "local", "tag": "dns-local"},
		direct,
		remote,
	}

	outbounds := []any{map[string]any{"type": "direct", "tag": "direct"}}
	var endpoints []any
	var warnings []string

	switch s.Mode {
	case settings.ModeWarp:
		if s.Warp == nil || s.Warp.PrivateKey == "" {
			return nil, nil, fmt.Errorf("chưa đăng ký WARP")
		}
		endpoints = append(endpoints, warpEndpoint(s.Warp))
	case settings.ModeCustom:
		obs, w, err := serverOutbounds(s)
		if err != nil {
			return nil, nil, err
		}
		outbounds = append(outbounds, obs...)
		warnings = w
	default:
		return nil, nil, fmt.Errorf("mode không hợp lệ: %q", s.Mode)
	}

	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
	}
	if s.BypassLAN {
		rules = append(rules, map[string]any{"ip_is_private": true, "outbound": "direct"})
	}
	var dnsRules []any
	var ruleSets []any

	r := s.Routing
	if procs := cleanList(r.DirectProcesses); len(procs) > 0 {
		rules = append(rules, map[string]any{"process_name": procs, "outbound": "direct"})
	}
	if doms := cleanDomains(r.ProxyDomains); len(doms) > 0 {
		rules = append(rules, map[string]any{"domain_suffix": doms, "outbound": TagProxy})
		dnsRules = append(dnsRules, map[string]any{"domain_suffix": doms, "server": "dns-remote"})
	}
	directDoms := cleanDomains(r.DirectDomains)
	if r.DirectVN {
		directDoms = append(directDoms, "vn")
	}
	if len(directDoms) > 0 {
		rules = append(rules, map[string]any{"domain_suffix": directDoms, "outbound": "direct"})
		// Resolve locally so CDNs hand out a nearby address.
		dnsRules = append(dnsRules, map[string]any{"domain_suffix": directDoms, "server": "dns-direct"})
	}
	if r.DirectVN {
		ruleSets = append(ruleSets, geoipVN(o.BinDir))
		rules = append(rules, map[string]any{"rule_set": "geoip-vn", "outbound": "direct"})
	}

	dns := map[string]any{
		"servers":  servers,
		"final":    "dns-remote",
		"strategy": "prefer_ipv4",
	}
	if len(dnsRules) > 0 {
		dns["rules"] = dnsRules
	}
	route := map[string]any{
		"rules":                   rules,
		"final":                   TagProxy,
		"auto_detect_interface":   true,
		"default_domain_resolver": "dns-direct",
	}
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}

	cfg := map[string]any{
		"log":       map[string]any{"level": "info", "timestamp": true},
		"dns":       dns,
		"inbounds":  []any{tunInbound()},
		"outbounds": outbounds,
		"route":     route,
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}
	if exp := experimental(o); len(exp) > 0 {
		cfg["experimental"] = exp
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	return data, warnings, err
}

// BuildKillSwitch returns a config that keeps the TUN up but rejects all
// traffic (except LAN, if allowed), so nothing leaks while the tunnel is down.
func BuildKillSwitch(s settings.Settings) ([]byte, error) {
	var rules []any
	if s.BypassLAN {
		rules = append(rules, map[string]any{"ip_is_private": true, "outbound": "direct"})
	}
	rules = append(rules, map[string]any{"action": "reject"})
	cfg := map[string]any{
		"log":       map[string]any{"level": "warn", "timestamp": true},
		"inbounds":  []any{tunInbound()},
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
		"route": map[string]any{
			"rules":                 rules,
			"final":                 "direct",
			"auto_detect_interface": true,
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func tunInbound() map[string]any {
	tun := map[string]any{
		"type":           "tun",
		"tag":            "tun-in",
		"interface_name": "BlackCyclone",
		"address":        []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
		"mtu":            9000,
		"auto_route":     true,
		"strict_route":   true,
		"stack":          "mixed",
	}
	platformTun(tun)
	return tun
}

func experimental(o Options) map[string]any {
	exp := map[string]any{}
	if o.CacheFile != "" {
		exp["cache_file"] = map[string]any{"enabled": true, "path": o.CacheFile}
	}
	if o.ClashAPI != "" {
		exp["clash_api"] = map[string]any{"external_controller": o.ClashAPI, "secret": o.ClashSecret}
	}
	return exp
}

// serverOutbounds builds one outbound per server, an "auto" urltest and the
// "proxy" selector that defaults to the user's choice.
func serverOutbounds(s settings.Settings) ([]any, []string, error) {
	var obs []any
	var tags, warnings []string
	selected := ""
	for _, srv := range s.Servers {
		tag := ServerTag(srv.ID)
		ob, err := ParseLink(srv.Link, tag)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("bỏ qua server %q: %v", srv.Name, err))
			continue
		}
		obs = append(obs, ob)
		tags = append(tags, tag)
		if srv.ID == s.SelectedServer {
			selected = tag
		}
	}
	if len(tags) == 0 {
		if len(warnings) > 0 {
			return nil, nil, fmt.Errorf("không có server dùng được: %s", warnings[0])
		}
		return nil, nil, fmt.Errorf("chưa có server nào: hãy thêm link hoặc subscription")
	}
	choices := tags
	if len(tags) > 1 {
		obs = append(obs, map[string]any{
			"type":      "urltest",
			"tag":       TagAuto,
			"outbounds": tags,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "3m",
			"tolerance": 50,
			// Same choice as the selector: when auto picks another server, move live connections too.
			"interrupt_exist_connections": s.InterruptOnSwitch,
		})
		choices = append([]string{TagAuto}, tags...)
		if selected == "" {
			selected = TagAuto
		}
	}
	if selected == "" {
		selected = tags[0]
	}
	obs = append(obs, map[string]any{
		"type":                        "selector",
		"tag":                         TagProxy,
		"outbounds":                   choices,
		"default":                     selected,
		"interrupt_exist_connections": s.InterruptOnSwitch,
	})
	return obs, warnings, nil
}

func geoipVN(binDir string) map[string]any {
	if binDir != "" {
		p := filepath.Join(binDir, "geoip-vn.srs")
		if _, err := os.Stat(p); err == nil {
			return map[string]any{"type": "local", "tag": "geoip-vn", "format": "binary", "path": p}
		}
	}
	return map[string]any{
		"type": "remote", "tag": "geoip-vn", "format": "binary",
		"url": geoipVNURL, "download_detour": "direct", "update_interval": "7d",
	}
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
		"tag":         TagProxy,
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

// cleanDomains normalises user input like "*.example.com", ".example.com",
// "https://example.com/x" into "example.com".
func cleanDomains(in []string) []string {
	var out []string
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		if i := strings.Index(d, "://"); i >= 0 {
			d = d[i+3:]
		}
		if i := strings.IndexAny(d, "/:"); i >= 0 {
			d = d[:i]
		}
		d = strings.TrimLeft(d, "*.")
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func cleanList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func clone(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
