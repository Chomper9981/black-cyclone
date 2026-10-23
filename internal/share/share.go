// Package share parses proxy share links (vless://, vmess://, trojan://, ss://,
// hysteria2:// / hy2://, tuic://) into sing-box (1.12+ format) outbounds.
package share

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// schemes lists the supported link schemes (lowercase, without "://").
var schemes = []string{"vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic"}

// ParseOutbound turns a share link into a sing-box outbound map with the given tag.
func ParseOutbound(link, tag string) (map[string]any, error) {
	link = strings.TrimSpace(link)
	var (
		out map[string]any
		err error
	)
	switch scheme(link) {
	case "vless":
		out, err = parseVLESS(link)
	case "vmess":
		out, err = parseVMess(link)
	case "trojan":
		out, err = parseTrojan(link)
	case "ss":
		out, err = parseSS(link)
	case "hysteria2", "hy2":
		out, err = parseHysteria2(link)
	case "tuic":
		out, err = parseTUIC(link)
	default:
		s := scheme(link)
		if s == "" {
			return nil, fmt.Errorf("not a share link")
		}
		return nil, fmt.Errorf("unsupported link scheme %q", s)
	}
	if err != nil {
		return nil, err
	}
	out["tag"] = tag
	return out, nil
}

// Name returns a display name for the link: the URL fragment, vmess "ps", or host:port.
func Name(link string) string {
	link = strings.TrimSpace(link)
	if scheme(link) == "vmess" {
		if v, err := decodeVMess(link); err == nil {
			if ps := str(v["ps"]); ps != "" {
				return ps
			}
		}
	} else if i := strings.IndexByte(link, '#'); i >= 0 {
		frag := link[i+1:]
		if s, err := url.PathUnescape(frag); err == nil {
			frag = s
		}
		if frag = strings.TrimSpace(frag); frag != "" {
			return frag
		}
	}
	if ob, err := ParseOutbound(link, "x"); err == nil {
		if h, ok := ob["server"].(string); ok && h != "" {
			if p, ok := ob["server_port"].(int); ok {
				return net.JoinHostPort(h, strconv.Itoa(p))
			}
			return h
		}
	}
	if s := scheme(link); s != "" {
		return s
	}
	return "Server"
}

// ParseLinks extracts supported share links from multi-line text, skipping blank
// lines, comments and unknown schemes. Several links may share a line when separated
// by whitespace; a link's fragment (name) may itself contain spaces.
func ParseLinks(text string) []string {
	var links []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		// Start offsets of whitespace-separated words that begin with a supported scheme.
		var starts []int
		for i := 0; i < len(line); i++ {
			if (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') && supported(scheme(line[i:])) {
				starts = append(starts, i)
			}
		}
		for n, s := range starts {
			end := len(line)
			if n+1 < len(starts) {
				end = starts[n+1]
			}
			links = append(links, strings.TrimSpace(line[s:end]))
		}
	}
	return links
}

func supported(s string) bool {
	for _, x := range schemes {
		if s == x {
			return true
		}
	}
	return false
}

// scheme returns the lowercase scheme of link ("" if it has none).
func scheme(link string) string {
	i := strings.Index(link, "://")
	if i <= 0 {
		return ""
	}
	return strings.ToLower(link[:i])
}

// ---------------------------------------------------------------- vless / trojan

func parseVLESS(link string) (map[string]any, error) {
	u, host, port, err := parseURL(link, "vless://UUID@host:port?...")
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	if uuid == "" {
		return nil, fmt.Errorf("link must look like vless://UUID@host:port?...")
	}
	q := u.Query()
	out := map[string]any{
		"type":        "vless",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}
	if flow := q.Get("flow"); flow != "" {
		out["flow"] = flow
	}
	if pe := q.Get("packetEncoding"); pe != "" && pe != "none" {
		out["packet_encoding"] = pe
	}
	if err := applyStream(out, q, host, q.Get("security")); err != nil {
		return nil, err
	}
	return out, nil
}

func parseTrojan(link string) (map[string]any, error) {
	u, host, port, err := parseURL(link, "trojan://password@host:port?...")
	if err != nil {
		return nil, err
	}
	pass := u.User.Username()
	if p, ok := u.User.Password(); ok {
		pass += ":" + p
	}
	if pass == "" {
		return nil, fmt.Errorf("link must look like trojan://password@host:port?...")
	}
	q := u.Query()
	out := map[string]any{
		"type":        "trojan",
		"server":      host,
		"server_port": port,
		"password":    pass,
	}
	sec := q.Get("security")
	if sec == "" {
		sec = "tls" // trojan is TLS unless explicitly disabled
	}
	if err := applyStream(out, q, host, sec); err != nil {
		return nil, err
	}
	return out, nil
}

// parseURL parses a userinfo@host:port URL and validates host and port.
func parseURL(link, shape string) (*url.URL, string, int, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, "", 0, err
	}
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if u.User == nil || host == "" || err != nil || port <= 0 || port > 65535 {
		return nil, "", 0, fmt.Errorf("link must look like %s", shape)
	}
	return u, host, port, nil
}

// applyStream adds tls and transport options from Xray-style query parameters.
func applyStream(out map[string]any, q url.Values, host, sec string) error {
	switch sec {
	case "reality", "tls", "xtls":
		tls := tlsOptions(firstNonEmpty(q.Get("sni"), q.Get("peer"), host), insecure(q), q.Get("alpn"))
		tls["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": firstNonEmpty(q.Get("fp"), "chrome"),
		}
		if sec == "reality" {
			if q.Get("pbk") == "" {
				return fmt.Errorf("reality link is missing pbk (public key)")
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
		return fmt.Errorf("unsupported security %q", sec)
	}

	tr, err := transport(q.Get("type"), q.Get("host"), q.Get("path"), q.Get("serviceName"), q.Get("ed"), q.Get("headerType"), host)
	if err != nil {
		return err
	}
	if tr != nil {
		out["transport"] = tr
	}
	return nil
}

// transport builds a sing-box V2Ray transport. A nil map means plain TCP.
func transport(typ, hostHdr, path, serviceName, ed, headerType, server string) (map[string]any, error) {
	switch strings.ToLower(typ) {
	case "", "tcp", "raw":
		if headerType == "http" {
			// TCP with HTTP header obfuscation maps onto the plain HTTP transport (no TLS).
			return httpTransport(hostHdr, path), nil
		}
		return nil, nil
	case "ws", "websocket":
		path, early := splitEarlyData(firstNonEmpty(path, "/"))
		if n, err := strconv.Atoi(ed); err == nil && n > 0 {
			early = n
		}
		t := map[string]any{
			"type":    "ws",
			"path":    path,
			"headers": map[string]any{"Host": firstNonEmpty(hostHdr, server)},
		}
		if early > 0 {
			t["max_early_data"] = early
			t["early_data_header_name"] = "Sec-WebSocket-Protocol"
		}
		return t, nil
	case "httpupgrade":
		t := map[string]any{"type": "httpupgrade", "path": firstNonEmpty(path, "/")}
		if hostHdr != "" {
			t["host"] = hostHdr
		}
		return t, nil
	case "grpc", "gun":
		return map[string]any{"type": "grpc", "service_name": firstNonEmpty(serviceName, path)}, nil
	case "http", "h2":
		return httpTransport(hostHdr, path), nil
	case "quic":
		return map[string]any{"type": "quic"}, nil
	case "xhttp", "splithttp":
		return nil, fmt.Errorf("transport %q (XHTTP/SplitHTTP) is not supported by sing-box; use ws, grpc, httpupgrade or tcp", typ)
	default:
		return nil, fmt.Errorf("unsupported transport %q", typ)
	}
}

func httpTransport(hostHdr, path string) map[string]any {
	t := map[string]any{"type": "http"}
	if hosts := splitList(hostHdr); len(hosts) > 0 {
		t["host"] = hosts
	}
	if path != "" {
		t["path"] = path
	}
	return t
}

// splitEarlyData removes "ed=N" from a websocket path such as "/ws?ed=2048".
func splitEarlyData(path string) (string, int) {
	i := strings.IndexByte(path, '?')
	if i < 0 {
		return path, 0
	}
	vals, err := url.ParseQuery(path[i+1:])
	if err != nil {
		return path, 0
	}
	n, err := strconv.Atoi(vals.Get("ed"))
	if err != nil || n <= 0 {
		return path, 0
	}
	vals.Del("ed")
	p := path[:i]
	if rest := vals.Encode(); rest != "" {
		p += "?" + rest
	}
	return firstNonEmpty(p, "/"), n
}

// ---------------------------------------------------------------- vmess

func decodeVMess(link string) (map[string]any, error) {
	body := link[len("vmess://"):]
	if i := strings.IndexByte(body, '#'); i >= 0 {
		body = body[:i]
	}
	raw, err := decodeBase64(body)
	if err != nil {
		return nil, fmt.Errorf("vmess link is not base64: %w", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("vmess link is not valid JSON: %w", err)
	}
	return v, nil
}

func parseVMess(link string) (map[string]any, error) {
	v, err := decodeVMess(link)
	if err != nil {
		return nil, err
	}
	host := str(v["add"])
	port, perr := strconv.Atoi(str(v["port"]))
	uuid := str(v["id"])
	if host == "" || uuid == "" || perr != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("vmess link is missing add/port/id")
	}
	aid, _ := strconv.Atoi(firstNonEmpty(str(v["aid"]), "0"))
	out := map[string]any{
		"type":        "vmess",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
		"security":    firstNonEmpty(str(v["scy"]), "auto"),
		"alter_id":    aid,
	}
	switch tls := str(v["tls"]); tls {
	case "tls":
		t := tlsOptions(firstNonEmpty(str(v["sni"]), str(v["host"]), host), truthy(str(v["allowInsecure"])) || truthy(str(v["insecure"])), str(v["alpn"]))
		t["utls"] = map[string]any{"enabled": true, "fingerprint": firstNonEmpty(str(v["fp"]), "chrome")}
		out["tls"] = t
	case "", "none":
	default:
		return nil, fmt.Errorf("unsupported vmess tls %q", tls)
	}
	path := str(v["path"])
	tr, err := transport(str(v["net"]), str(v["host"]), path, path, "", str(v["type"]), host)
	if err != nil {
		return nil, err
	}
	if tr != nil {
		out["transport"] = tr
	}
	return out, nil
}

// ---------------------------------------------------------------- shadowsocks

func parseSS(link string) (map[string]any, error) {
	body := link[len("ss://"):]
	if i := strings.IndexByte(body, '#'); i >= 0 {
		body = body[:i]
	}
	var query url.Values
	if i := strings.IndexByte(body, '?'); i >= 0 {
		query, _ = url.ParseQuery(body[i+1:])
		body = body[:i]
	}
	body = strings.TrimSuffix(body, "/")

	var userinfo, hostport string
	if at := strings.LastIndexByte(body, '@'); at >= 0 {
		// SIP002: base64url(method:password)@host:port, or percent-encoded method:password.
		userinfo, hostport = body[:at], body[at+1:]
		if s, err := url.PathUnescape(userinfo); err == nil {
			userinfo = s
		}
		if raw, err := decodeBase64(userinfo); err == nil && strings.Contains(string(raw), ":") {
			userinfo = string(raw)
		}
	} else {
		// Legacy: base64(method:password@host:port).
		raw, err := decodeBase64(body)
		if err != nil {
			return nil, fmt.Errorf("ss link is not valid: %w", err)
		}
		s := string(raw)
		at := strings.LastIndexByte(s, '@')
		if at < 0 {
			return nil, fmt.Errorf("ss link must look like ss://method:password@host:port")
		}
		userinfo, hostport = s[:at], s[at+1:]
	}
	method, password, ok := strings.Cut(userinfo, ":")
	host, portStr, err := net.SplitHostPort(hostport)
	port, perr := strconv.Atoi(portStr)
	if !ok || method == "" || err != nil || host == "" || perr != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("ss link must look like ss://method:password@host:port")
	}
	out := map[string]any{
		"type":        "shadowsocks",
		"server":      host,
		"server_port": port,
		"method":      strings.ToLower(method),
		"password":    password,
	}
	if plugin := query.Get("plugin"); plugin != "" {
		name, opts, _ := strings.Cut(plugin, ";")
		switch name {
		case "obfs-local", "simple-obfs":
			out["plugin"] = "obfs-local"
		case "v2ray-plugin":
			out["plugin"] = "v2ray-plugin"
		default:
			return nil, fmt.Errorf("unsupported shadowsocks plugin %q (only obfs-local and v2ray-plugin)", name)
		}
		if opts != "" {
			out["plugin_opts"] = opts
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- hysteria2 / tuic

func parseHysteria2(link string) (map[string]any, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	port := 443
	if p := u.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid hysteria2 port %q", p)
		}
	}
	if host == "" {
		return nil, fmt.Errorf("link must look like hysteria2://auth@host:port?...")
	}
	q := u.Query()
	out := map[string]any{
		"type":        "hysteria2",
		"server":      host,
		"server_port": port,
	}
	if u.User != nil {
		auth := u.User.Username()
		if p, ok := u.User.Password(); ok {
			auth += ":" + p
		}
		if auth != "" {
			out["password"] = auth
		}
	}
	if pw := q.Get("auth"); pw != "" {
		out["password"] = pw
	}
	if mport := q.Get("mport"); mport != "" {
		var ports []string
		for _, r := range splitList(mport) {
			ports = append(ports, strings.Replace(r, "-", ":", 1))
		}
		out["server_ports"] = ports
	}
	switch obfs := q.Get("obfs"); obfs {
	case "", "none":
	case "salamander":
		out["obfs"] = map[string]any{"type": "salamander", "password": q.Get("obfs-password")}
	default:
		return nil, fmt.Errorf("unsupported hysteria2 obfs %q", obfs)
	}
	if n := leadingInt(q.Get("up")); n > 0 {
		out["up_mbps"] = n
	}
	if n := leadingInt(q.Get("down")); n > 0 {
		out["down_mbps"] = n
	}
	out["tls"] = tlsOptions(firstNonEmpty(q.Get("sni"), host), insecure(q), q.Get("alpn"))
	return out, nil
}

func parseTUIC(link string) (map[string]any, error) {
	u, host, port, err := parseURL(link, "tuic://uuid:password@host:port?...")
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	pass, _ := u.User.Password()
	if uuid == "" {
		return nil, fmt.Errorf("link must look like tuic://uuid:password@host:port?...")
	}
	q := u.Query()
	out := map[string]any{
		"type":        "tuic",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
		"password":    pass,
	}
	if cc := firstNonEmpty(q.Get("congestion_control"), q.Get("congestion-control")); cc != "" {
		out["congestion_control"] = cc
	}
	if m := firstNonEmpty(q.Get("udp_relay_mode"), q.Get("udp-relay-mode")); m != "" {
		out["udp_relay_mode"] = m
	}
	tls := tlsOptions(firstNonEmpty(q.Get("sni"), host), insecure(q), firstNonEmpty(q.Get("alpn"), "h3"))
	if truthy(q.Get("disable_sni")) {
		tls["disable_sni"] = true
	}
	out["tls"] = tls
	return out, nil
}

// ---------------------------------------------------------------- helpers

func tlsOptions(sni string, insecure bool, alpn string) map[string]any {
	t := map[string]any{"enabled": true, "server_name": sni}
	if insecure {
		t["insecure"] = true
	}
	if a := splitList(alpn); len(a) > 0 {
		t["alpn"] = a
	}
	return t
}

func insecure(q url.Values) bool {
	return truthy(q.Get("allowInsecure")) || truthy(q.Get("insecure")) || truthy(q.Get("allow_insecure"))
}

func truthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}

// decodeBase64 accepts standard or URL-safe base64, with or without padding and
// with embedded whitespace.
func decodeBase64(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, s)
	s = strings.TrimRight(s, "=")
	if strings.ContainsAny(s, "-_") {
		return base64.RawURLEncoding.DecodeString(s)
	}
	return base64.RawStdEncoding.DecodeString(s)
}

// DecodeBase64 is decodeBase64 exported for the subscription package.
func DecodeBase64(s string) ([]byte, error) { return decodeBase64(s) }

func str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(s[:i])
	return n
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
