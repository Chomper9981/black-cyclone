package share

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const uuid = "11111111-2222-3333-4444-555555555555"

func vmessLink(fields map[string]any, enc *base64.Encoding) string {
	b, _ := json.Marshal(fields)
	return "vmess://" + enc.EncodeToString(b)
}

type sample struct {
	name string
	link string
	want map[string]any // subset of fields checked (dotted paths)
}

func samples() []sample {
	return []sample{
		{"vless-reality", "vless://" + uuid + "@vps.example.com:443?security=reality&sni=www.microsoft.com&pbk=bmXOC-F1FxEMF9dyiK2H5_1SUtzH0JuVo51h2wPfgyo&sid=abcd&fp=chrome&flow=xtls-rprx-vision&type=tcp#vps",
			map[string]any{"type": "vless", "server": "vps.example.com", "server_port": 443, "flow": "xtls-rprx-vision", "tls.server_name": "www.microsoft.com", "tls.reality.short_id": "abcd", "tls.utls.fingerprint": "chrome"}},
		{"vless-ws-tls-ed-query", "vless://" + uuid + "@1.2.3.4:443?security=tls&sni=cdn.example.com&alpn=h2,http/1.1&allowInsecure=1&type=ws&host=cdn.example.com&path=%2Fws&ed=2048&packetEncoding=xudp#ws",
			map[string]any{"transport.type": "ws", "transport.path": "/ws", "transport.max_early_data": 2048, "transport.early_data_header_name": "Sec-WebSocket-Protocol", "transport.headers.Host": "cdn.example.com", "tls.insecure": true, "tls.alpn": []string{"h2", "http/1.1"}, "packet_encoding": "xudp"}},
		{"vless-ws-ed-in-path", "vless://" + uuid + "@1.2.3.4:80?type=ws&path=%2Fray%3Fed%3D2048",
			map[string]any{"transport.path": "/ray", "transport.max_early_data": 2048}},
		{"vless-grpc", "vless://" + uuid + "@g.example.com:443?security=tls&type=grpc&serviceName=gsvc&insecure=1",
			map[string]any{"transport.type": "grpc", "transport.service_name": "gsvc", "tls.insecure": true}},
		{"vless-httpupgrade", "vless://" + uuid + "@h.example.com:443?security=tls&type=httpupgrade&host=h.example.com&path=%2Fup",
			map[string]any{"transport.type": "httpupgrade", "transport.host": "h.example.com", "transport.path": "/up"}},
		{"vless-h2", "vless://" + uuid + "@h.example.com:443?security=tls&type=http&host=a.com,b.com&path=%2Fh2",
			map[string]any{"transport.type": "http", "transport.host": []string{"a.com", "b.com"}, "transport.path": "/h2"}},
		{"vless-plain", "vless://" + uuid + "@10.0.0.1:8080?encryption=none", map[string]any{"type": "vless", "server_port": 8080}},
		{"vmess-ws-tls", vmessLink(map[string]any{"v": "2", "ps": "VN vmess", "add": "vm.example.com", "port": "443", "id": uuid, "aid": "0", "scy": "auto", "net": "ws", "type": "none", "host": "vm.example.com", "path": "/vm?ed=2048", "tls": "tls", "sni": "vm.example.com", "alpn": "http/1.1", "fp": "firefox"}, base64.StdEncoding),
			map[string]any{"type": "vmess", "server": "vm.example.com", "server_port": 443, "security": "auto", "alter_id": 0, "transport.type": "ws", "transport.path": "/vm", "transport.max_early_data": 2048, "tls.utls.fingerprint": "firefox", "tls.alpn": []string{"http/1.1"}}},
		{"vmess-tcp-numeric-port-rawurl", vmessLink(map[string]any{"ps": "n", "add": "1.2.3.4", "port": 10086, "id": uuid, "aid": 0, "net": "tcp"}, base64.RawURLEncoding),
			map[string]any{"server_port": 10086, "security": "auto"}},
		{"vmess-grpc", vmessLink(map[string]any{"add": "1.2.3.4", "port": "443", "id": uuid, "net": "grpc", "path": "svc", "tls": "tls"}, base64.RawStdEncoding),
			map[string]any{"transport.type": "grpc", "transport.service_name": "svc"}},
		{"trojan", "trojan://p%40ss@tr.example.com:443?sni=sni.example.com&allowInsecure=1#trojan%20node",
			map[string]any{"type": "trojan", "password": "p@ss", "tls.server_name": "sni.example.com", "tls.insecure": true}},
		{"trojan-ws", "trojan://pass@tr.example.com:443?security=tls&type=ws&path=%2Ftr#t",
			map[string]any{"transport.type": "ws", "transport.path": "/tr"}},
		{"ss-sip002-base64", "ss://" + base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret")) + "@ss.example.com:8388#ss1",
			map[string]any{"type": "shadowsocks", "method": "chacha20-ietf-poly1305", "password": "secret", "server_port": 8388}},
		{"ss-sip002-padded-escaped", "ss://" + strings.ReplaceAll(base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw1")), "=", "%3D") + "@1.2.3.4:443",
			map[string]any{"method": "aes-256-gcm", "password": "pw1"}},
		{"ss-2022-plain", "ss://2022-blake3-aes-128-gcm:" + "AAAAAAAAAAAAAAAAAAAAAA%3D%3D" + "@1.2.3.4:443#ss22",
			map[string]any{"method": "2022-blake3-aes-128-gcm", "password": "AAAAAAAAAAAAAAAAAAAAAA=="}},
		{"ss-legacy", "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:pass@legacy.example.com:8388")) + "#legacy",
			map[string]any{"server": "legacy.example.com", "method": "aes-128-gcm", "password": "pass"}},
		{"ss-obfs", "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:pass")) + "@1.2.3.4:443/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dexample.com#o",
			map[string]any{"plugin": "obfs-local", "plugin_opts": "obfs=http;obfs-host=example.com"}},
		{"hysteria2", "hysteria2://letmein@hy.example.com:8443/?sni=real.example.com&insecure=1&obfs=salamander&obfs-password=obfspw&up=50&down=200%20mbps#hy",
			map[string]any{"type": "hysteria2", "password": "letmein", "server_port": 8443, "obfs.type": "salamander", "obfs.password": "obfspw", "up_mbps": 50, "down_mbps": 200, "tls.server_name": "real.example.com", "tls.insecure": true}},
		{"hy2-default-port", "hy2://pw@hy.example.com?mport=20000-30000",
			map[string]any{"server_port": 443, "server_ports": []string{"20000:30000"}, "tls.server_name": "hy.example.com"}},
		{"tuic", "tuic://" + uuid + ":tpass@tuic.example.com:443?congestion_control=bbr&alpn=h3&udp_relay_mode=native&sni=tuic.example.com&allow_insecure=1#tuic",
			map[string]any{"type": "tuic", "uuid": uuid, "password": "tpass", "congestion_control": "bbr", "udp_relay_mode": "native", "tls.alpn": []string{"h3"}, "tls.insecure": true}},
	}
}

func lookup(m map[string]any, path string) any {
	var cur any = m
	for _, k := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func TestParseOutbound(t *testing.T) {
	for _, s := range samples() {
		t.Run(s.name, func(t *testing.T) {
			ob, err := ParseOutbound(s.link, "proxy")
			if err != nil {
				t.Fatal(err)
			}
			if ob["tag"] != "proxy" {
				t.Errorf("tag = %v", ob["tag"])
			}
			for k, want := range s.want {
				if got := lookup(ob, k); !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %#v, want %#v", k, got, want)
				}
			}
		})
	}
}

func TestParseOutboundErrors(t *testing.T) {
	bad := map[string]string{
		"xhttp":        "vless://" + uuid + "@h.com:443?security=tls&type=xhttp",
		"splithttp":    "vless://" + uuid + "@h.com:443?type=splithttp",
		"reality-pbk":  "vless://" + uuid + "@h.com:443?security=reality",
		"no-port":      "vless://" + uuid + "@h.com",
		"unknown":      "socks://h.com:1080",
		"garbage":      "hello",
		"vmess-b64":    "vmess://!!!",
		"ss-plugin":    "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:p")) + "@h.com:1?plugin=kcptun",
		"hy2-obfs":     "hy2://pw@h.com:443?obfs=xor",
		"tuic-no-user": "tuic://h.com:443",
	}
	for name, link := range bad {
		if _, err := ParseOutbound(link, "proxy"); err == nil {
			t.Errorf("%s: expected error for %s", name, link)
		}
	}
	_, err := ParseOutbound("vless://"+uuid+"@h.com:443?type=xhttp", "proxy")
	if err == nil || !strings.Contains(err.Error(), "not supported by sing-box") {
		t.Errorf("xhttp error should be explicit, got %v", err)
	}
}

func TestName(t *testing.T) {
	cases := map[string]string{
		"vless://" + uuid + "@h.com:443#My%20Server": "My Server",
		"trojan://p@h.com:443":                       "h.com:443",
		vmessLink(map[string]any{"ps": "VN 1", "add": "a", "port": "1", "id": uuid}, base64.StdEncoding): "VN 1",
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:p@1.2.3.4:8388")):                "1.2.3.4:8388",
		"hy2://pw@[2001:db8::1]:443": "[2001:db8::1]:443",
	}
	for link, want := range cases {
		if got := Name(link); got != want {
			t.Errorf("Name(%q) = %q, want %q", link, got, want)
		}
	}
}

func TestParseLinks(t *testing.T) {
	text := "# comment\r\n\r\nvless://a@h:1#x\r\n  trojan://p@h:2  \nhttp://example.com\nfoo bar\n// note\nHY2://p@h:3 ss://abc@h:4\n"
	got := ParseLinks(text)
	want := []string{"vless://a@h:1#x", "trojan://p@h:2", "HY2://p@h:3", "ss://abc@h:4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodeBase64Variants(t *testing.T) {
	plain := "method:pass?>>"
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		s := enc.EncodeToString([]byte(plain))
		s = s[:4] + "\n" + s[4:]
		got, err := DecodeBase64(s)
		if err != nil || string(got) != plain {
			t.Errorf("decode %q: %q %v", s, got, err)
		}
	}
}

// TestCheckWithSingBox validates every parsed sample with the real `sing-box check`.
func TestCheckWithSingBox(t *testing.T) {
	bin := os.Getenv("SING_BOX")
	if bin == "" {
		bin, _ = filepath.Abs(filepath.Join("..", "..", "bin", "sing-box.exe"))
	}
	if _, err := os.Stat(bin); err != nil {
		bin = `D:\Projects\redcyclone-vpn\bin\sing-box.exe`
		if _, err := os.Stat(bin); err != nil {
			t.Skip("sing-box.exe not found; set SING_BOX or run scripts/fetch-bins.ps1")
		}
	}
	for _, s := range samples() {
		t.Run(s.name, func(t *testing.T) {
			ob, err := ParseOutbound(s.link, "proxy")
			if err != nil {
				t.Fatal(err)
			}
			cfg, _ := json.MarshalIndent(map[string]any{
				"outbounds": []any{ob, map[string]any{"type": "direct", "tag": "direct"}},
			}, "", "  ")
			f := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(f, cfg, 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(bin, "check", "-c", f).CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed: %v\n%s\n%s", err, out, cfg)
			}
		})
	}
}
