package singbox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"redcyclone-vpn/internal/settings"
)

const realityLink = "vless://11111111-2222-3333-4444-555555555555@vps.example.com:443?security=reality&sni=www.microsoft.com&pbk=bmXOC-F1FxEMF9dyiK2H5_1SUtzH0JuVo51h2wPfgyo&sid=abcd&fp=chrome&flow=xtls-rprx-vision&type=tcp#vps"

func singBox(t *testing.T) string {
	bin, err := filepath.Abs(filepath.Join("..", "..", "bin", "sing-box.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("bin/sing-box.exe not found; run scripts/fetch-bins.ps1")
	}
	return bin
}

func check(t *testing.T, bin string, cfg []byte) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(f, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "check", "-c", f).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\n%s", err, out, cfg)
	}
}

// TestCheckWithSingBox validates every mode's config with the real `sing-box check`.
func TestCheckWithSingBox(t *testing.T) {
	bin := singBox(t)

	base := settings.Default()
	warp := base
	warp.Mode = settings.ModeWarp
	warp.Warp = &settings.Warp{
		PrivateKey:    "aCs1dJ2Ld4VFKP2BSEFUOR1DvhPn3a5ofkT+QeaUgVo=",
		PeerPublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		Endpoint:      "162.159.192.1",
		EndpointPort:  2408,
		Addresses:     []string{"172.16.0.2/32", "2606:4700:110:8a36::1/128"},
		Reserved:      []int{1, 2, 3},
	}
	warp.Routing = settings.Routing{
		DirectVN:        true,
		DirectDomains:   []string{"*.zalo.me", "https://vnexpress.net/abc"},
		ProxyDomains:    []string{"facebook.com"},
		DirectProcesses: []string{"steam.exe"},
	}
	one := base
	one.Mode = settings.ModeCustom
	one.Servers = []settings.Server{{ID: "a", Name: "vps", Link: realityLink}}
	one.SelectedServer = "a"
	one.Routing = settings.Routing{}
	multi := one
	multi.Servers = append(multi.Servers,
		settings.Server{ID: "b", Name: "vps2", Link: strings.Replace(realityLink, "vps.example.com", "vps2.example.com", 1)},
		settings.Server{ID: "c", Name: "broken", Link: "vless://nope"})
	multi.SelectedServer = settings.AutoServer
	domainDoH := base
	domainDoH.DoHServer = "https://dns.google/dns-query"

	opts := Options{BinDir: t.TempDir(), CacheFile: filepath.Join(t.TempDir(), "cache.db"), ClashAPI: "127.0.0.1:9090", ClashSecret: "x"}
	cases := map[string]settings.Settings{"bypass": base, "warp": warp, "custom-one": one, "custom-multi": multi, "doh-domain": domainDoH}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, warnings, err := Build(s, opts)
			if err != nil {
				t.Fatal(err)
			}
			if name == "custom-multi" && len(warnings) != 1 {
				t.Errorf("want 1 warning for the broken link, got %v", warnings)
			}
			check(t, bin, cfg)
		})
	}
	t.Run("killswitch", func(t *testing.T) {
		cfg, err := BuildKillSwitch(base)
		if err != nil {
			t.Fatal(err)
		}
		check(t, bin, cfg)
	})
}

func TestSelector(t *testing.T) {
	s := settings.Default()
	s.Mode = settings.ModeCustom
	s.Servers = []settings.Server{{ID: "a", Link: realityLink}, {ID: "b", Link: realityLink}}
	s.SelectedServer = "b"
	cfg, _, err := Build(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, ob := range parsed.Outbounds {
		if ob["tag"] == TagProxy {
			if ob["default"] != ServerTag("b") {
				t.Fatalf("selector default = %v", ob["default"])
			}
			return
		}
	}
	t.Fatal("no proxy selector")
}

func TestNoServers(t *testing.T) {
	s := settings.Default()
	s.Mode = settings.ModeCustom
	if _, _, err := Build(s, Options{}); err == nil {
		t.Fatal("expected error with no servers")
	}
}

func TestCleanDomains(t *testing.T) {
	got := cleanDomains([]string{" *.Zalo.me ", "https://vnexpress.net/x", ".tiki.vn", "", "a.com:443"})
	want := []string{"zalo.me", "vnexpress.net", "tiki.vn", "a.com"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
}
