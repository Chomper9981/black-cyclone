package rcvcore

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"

	"black-cyclone/internal/settings"
	"black-cyclone/internal/singbox"
)

// Run with the build tags from mobile/build-apk.ps1, e.g.
//
//	go test -tags with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,badlinkname,tfogo_checklinkname0 -ldflags=-checklinkname=0 ./...

type fakePlatform struct{}

func (fakePlatform) Protect(int32) bool                                        { return true }
func (fakePlatform) Interfaces() string                                        { return "[]" }
func (fakePlatform) ConnectionOwner(int32, string, int32, string, int32) int32 { return -1 }

// TestConfigsLoad builds the configs the app produces and lets sing-box parse
// and construct them, which fails if a needed feature is not compiled in.
func TestConfigsLoad(t *testing.T) {
	warpAcc := &settings.Warp{
		PrivateKey:    "aGVsbG8taGVsbG8taGVsbG8taGVsbG8taGVsbG8tMTI=",
		PeerPublicKey: "bmV3LXBlZXItbmV3LXBlZXItbmV3LXBlZXItbmV3LTE=",
		Endpoint:      "162.159.192.1", EndpointPort: 2408,
		Addresses: []string{"172.16.0.2/32", "2606:4700:110:8a36::1/128"},
		Reserved:  []int{1, 2, 3},
	}
	links := []string{
		"vless://9d4e5f2a-1b3c-4d5e-8f90-123456789abc@203.0.113.10:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.microsoft.com&fp=chrome&pbk=SbVKOEMjK0sIlbwg4akyBg5mL5KZwwB-ed4eEE7YnRc&sid=6ba85179e30d4fc2&type=tcp#vps",
		"hysteria2://pass@203.0.113.11:443?sni=example.com#hy2",
		"trojan://pass@203.0.113.12:443?sni=example.com#trojan",
	}
	cases := map[string]settings.Settings{}
	w := settings.Default()
	w.Warp = warpAcc
	cases["warp"] = w
	c := settings.Default()
	c.Mode = settings.ModeCustom
	for i, l := range links {
		c.Servers = append(c.Servers, settings.Server{ID: string(rune('a' + i)), Name: "s", Link: l})
	}
	cases["custom"] = c

	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			raw, _ := json.Marshal(s)
			var conf settings.Settings
			if err := json.Unmarshal(raw, &conf); err != nil {
				t.Fatal(err)
			}
			cfg, warnings, err := singbox.Build(conf, singbox.Options{
				BinDir: dir, CacheFile: filepath.Join(dir, "cache.db"),
				ClashAPI: "127.0.0.1:0", ClashSecret: "x",
			})
			if err != nil || len(warnings) > 0 {
				t.Fatalf("build: %v %v", err, warnings)
			}
			ctx, cancel := context.WithCancel(include.Context(context.Background()))
			defer cancel()
			opts, err := sjson.UnmarshalExtendedContext[option.Options](ctx, cfg)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			ctx = service.ContextWith[adapter.PlatformInterface](ctx, &platform{p: fakePlatform{}, tunFd: -1})
			instance, err := box.New(box.Options{Context: ctx, Options: opts})
			if err != nil {
				t.Fatalf("box.New: %v", err)
			}
			instance.Close()
		})
	}
}

func TestParseLinks(t *testing.T) {
	var got []linkInfo
	in := "rác vless://9d4e5f2a-1b3c-4d5e-8f90-123456789abc@203.0.113.10:443?security=reality&sni=a.com&pbk=SbVKOEMjK0sIlbwg4akyBg5mL5KZwwB-ed4eEE7YnRc&type=tcp#nha\nvless://bad"
	if err := json.Unmarshal([]byte(ParseLinks(in)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "nha" || got[0].Error != "" || got[1].Error == "" {
		t.Fatalf("got %+v", got)
	}
	if !strings.HasPrefix(got[0].Link, "vless://") {
		t.Fatalf("link %q", got[0].Link)
	}
}
