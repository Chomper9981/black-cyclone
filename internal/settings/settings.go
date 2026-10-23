// Package settings persists user configuration under %APPDATA%\RedCyclone.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Mode string

const (
	// ModeBypass: no remote server. DoH DNS through a TUN + GoodbyeDPI against SNI/DPI blocking.
	ModeBypass Mode = "bypass"
	// ModeWarp: all traffic through Cloudflare WARP (WireGuard). Hides your IP from websites.
	ModeWarp Mode = "warp"
	// ModeCustom: all traffic through your own server given as a vless:// share link.
	ModeCustom Mode = "custom"
)

type Warp struct {
	PrivateKey    string   `json:"private_key"`
	PeerPublicKey string   `json:"peer_public_key"`
	Endpoint      string   `json:"endpoint"`
	EndpointPort  int      `json:"endpoint_port"`
	Addresses     []string `json:"addresses"`
	Reserved      []int    `json:"reserved"`
}

type Settings struct {
	Mode       Mode   `json:"mode"`
	DoHServer  string `json:"doh_server"`
	UseDPI     bool   `json:"use_dpi"`
	CustomLink string `json:"custom_link"`
	BypassLAN  bool   `json:"bypass_lan"`
	Warp       *Warp  `json:"warp,omitempty"`
}

func Default() Settings {
	return Settings{
		Mode:      ModeBypass,
		DoHServer: "https://1.1.1.1/dns-query",
		UseDPI:    true,
		BypassLAN: true,
	}
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "RedCyclone")
	return dir, os.MkdirAll(dir, 0o700)
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func Load() (Settings, error) {
	s := Default()
	p, err := path()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

func Save(s Settings) error {
	p, err := path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
