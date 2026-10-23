package singbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"redcyclone-vpn/internal/settings"
)

// TestCheckWithSingBox validates every mode's config with the real `sing-box check`.
func TestCheckWithSingBox(t *testing.T) {
	bin, err := filepath.Abs(filepath.Join("..", "..", "bin", "sing-box.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("bin/sing-box.exe not found; run scripts/fetch-bins.ps1")
	}

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
	custom := base
	custom.Mode = settings.ModeCustom
	custom.CustomLink = "vless://11111111-2222-3333-4444-555555555555@vps.example.com:443?security=reality&sni=www.microsoft.com&pbk=bmXOC-F1FxEMF9dyiK2H5_1SUtzH0JuVo51h2wPfgyo&sid=abcd&fp=chrome&flow=xtls-rprx-vision&type=tcp#vps"
	domainDoH := base
	domainDoH.DoHServer = "https://dns.google/dns-query"

	cases := map[string]settings.Settings{"bypass": base, "warp": warp, "custom": custom, "doh-domain": domainDoH}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := Build(s)
			if err != nil {
				t.Fatal(err)
			}
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
