// Package vless parses vless:// share links (Xray / sing-box format) into a sing-box outbound.
package vless

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseOutbound turns
//
//	vless://UUID@host:port?security=reality&sni=...&pbk=...&sid=...&fp=chrome&flow=xtls-rprx-vision&type=tcp#name
//
// into a sing-box outbound map tagged "proxy".
func ParseOutbound(link string) (map[string]any, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "vless" {
		return nil, fmt.Errorf("only vless:// links are supported, got %q", u.Scheme)
	}
	uuid := u.User.Username()
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if uuid == "" || host == "" || err != nil {
		return nil, fmt.Errorf("link must look like vless://UUID@host:port?...")
	}
	q := u.Query()

	out := map[string]any{
		"type":        "vless",
		"tag":         "proxy",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}
	if flow := q.Get("flow"); flow != "" {
		out["flow"] = flow
	}

	switch sec := q.Get("security"); sec {
	case "reality", "tls":
		tls := map[string]any{
			"enabled":     true,
			"server_name": firstNonEmpty(q.Get("sni"), host),
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": firstNonEmpty(q.Get("fp"), "chrome"),
			},
		}
		if sec == "reality" {
			if q.Get("pbk") == "" {
				return nil, fmt.Errorf("reality link is missing pbk (public key)")
			}
			tls["reality"] = map[string]any{
				"enabled":    true,
				"public_key": q.Get("pbk"),
				"short_id":   q.Get("sid"),
			}
		}
		out["tls"] = tls
	case "", "none":
	default:
		return nil, fmt.Errorf("unsupported security %q", sec)
	}

	switch typ := q.Get("type"); typ {
	case "", "tcp":
	case "ws":
		out["transport"] = map[string]any{
			"type":    "ws",
			"path":    firstNonEmpty(q.Get("path"), "/"),
			"headers": map[string]any{"Host": firstNonEmpty(q.Get("host"), host)},
		}
	case "grpc":
		out["transport"] = map[string]any{"type": "grpc", "service_name": q.Get("serviceName")}
	default:
		return nil, fmt.Errorf("unsupported transport %q", typ)
	}
	return out, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
