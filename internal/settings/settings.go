// Package settings persists user configuration under %APPDATA%\RedCyclone.
// The file is encrypted with Windows DPAPI (current user) because it holds
// WireGuard private keys and proxy credentials.
package settings

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Mode string

const (
	// ModeBypass: no remote server. DoH DNS through a TUN + GoodbyeDPI against SNI/DPI blocking.
	ModeBypass Mode = "bypass"
	// ModeWarp: all traffic through Cloudflare WARP (WireGuard). Hides your IP from websites.
	ModeWarp Mode = "warp"
	// ModeCustom: all traffic through your own server(s) given as share links / subscriptions.
	ModeCustom Mode = "custom"
)

// GoodbyeDPI presets offered in the UI. DPIArgs is used when DPIPreset == "custom".
var DPIPresets = map[string][]string{
	"5":  {"-5"},       // -f 2 -e 2 --auto-ttl --reverse-frag --max-payload
	"9":  {"-9"},       // -5 plus fake packets (wrong chksum/seq) and QUIC blocking
	"9q": {"-9", "-q"}, // explicit: also block QUIC so browsers fall back to TCP
}

type Warp struct {
	PrivateKey    string   `json:"private_key"`
	PeerPublicKey string   `json:"peer_public_key"`
	Endpoint      string   `json:"endpoint"`
	EndpointPort  int      `json:"endpoint_port"`
	Addresses     []string `json:"addresses"`
	Reserved      []int    `json:"reserved"`
	// Device ID and API token from registration, needed to apply a WARP+ license.
	DeviceID string `json:"device_id,omitempty"`
	Token    string `json:"token,omitempty"`
	License  string `json:"license,omitempty"`
	// AccountType as reported by the API: "free", "unlimited" (WARP+), ...
	AccountType string `json:"account_type,omitempty"`
}

// Server is one proxy server given as a share link (vless://, vmess://, trojan://, ss://, hysteria2://, tuic://).
type Server struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Link  string `json:"link"`
	SubID string `json:"sub_id,omitempty"` // owning subscription; empty = added by hand
}

type Subscription struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Updated string `json:"updated,omitempty"` // RFC3339 of last successful refresh
}

// Routing applies in the warp/custom modes (what bypasses the tunnel).
type Routing struct {
	DirectVN        bool     `json:"direct_vn"`        // Vietnamese IPs / .vn domains go direct
	DirectDomains   []string `json:"direct_domains"`   // domain suffixes that go direct
	ProxyDomains    []string `json:"proxy_domains"`    // domain suffixes forced through the tunnel (wins over DirectVN)
	DirectProcesses []string `json:"direct_processes"` // e.g. "steam.exe"
}

// AutoServer as SelectedServer picks the lowest-latency server automatically (urltest).
const AutoServer = "auto"

type Settings struct {
	Mode      Mode   `json:"mode"`
	DoHServer string `json:"doh_server"`
	BypassLAN bool   `json:"bypass_lan"`

	UseDPI    bool   `json:"use_dpi"`
	DPIPreset string `json:"dpi_preset"` // key of DPIPresets or "custom"
	DPIArgs   string `json:"dpi_args"`   // space-separated GoodbyeDPI args for "custom"

	Servers        []Server       `json:"servers"`
	Subscriptions  []Subscription `json:"subscriptions"`
	SelectedServer string         `json:"selected_server"` // Server.ID or AutoServer
	// Deprecated: single vless link from v1; migrated into Servers on load.
	CustomLink string `json:"custom_link,omitempty"`

	Warp *Warp `json:"warp,omitempty"`

	Routing Routing `json:"routing"`

	KillSwitch       bool `json:"kill_switch"`    // block traffic if the tunnel dies (warp/custom)
	AutoReconnect    bool `json:"auto_reconnect"` // restart the core if it dies
	AutoConnect      bool `json:"auto_connect"`   // connect when the app starts
	StartWithWindows bool `json:"start_with_windows"`
	StartMinimized   bool `json:"start_minimized"` // launch hidden in the tray
	CloseToTray      bool `json:"close_to_tray"`
}

func Default() Settings {
	return Settings{
		Mode:           ModeBypass,
		DoHServer:      "https://1.1.1.1/dns-query",
		UseDPI:         true,
		DPIPreset:      "9",
		BypassLAN:      true,
		SelectedServer: AutoServer,
		Routing:        Routing{DirectVN: true},
		KillSwitch:     true,
		AutoReconnect:  true,
		CloseToTray:    true,
	}
}

// DPICommand returns the GoodbyeDPI arguments for the configured preset.
func (s Settings) DPICommand() []string {
	if s.DPIPreset == "custom" {
		if f := strings.Fields(s.DPIArgs); len(f) > 0 {
			return f
		}
	}
	if a, ok := DPIPresets[s.DPIPreset]; ok {
		return a
	}
	return DPIPresets["9"]
}

// NewID returns a short random identifier for servers/subscriptions.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
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

// encMagic prefixes DPAPI-encrypted settings files; plain JSON (v1) starts with '{'.
const encMagic = "RCV1"

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
	if strings.HasPrefix(string(data), encMagic) {
		if data, err = unprotect(data[len(encMagic):]); err != nil {
			return s, errors.New("không giải mã được settings.json: " + err.Error())
		}
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	s.migrate()
	return s, nil
}

func (s *Settings) migrate() {
	if s.CustomLink != "" {
		s.Servers = append(s.Servers, Server{ID: NewID(), Name: "Server", Link: s.CustomLink})
		if s.SelectedServer == "" || s.SelectedServer == AutoServer {
			s.SelectedServer = s.Servers[len(s.Servers)-1].ID
		}
		s.CustomLink = ""
	}
	if s.SelectedServer == "" {
		s.SelectedServer = AutoServer
	}
	if s.DPIPreset == "" {
		s.DPIPreset = "9"
	}
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
	enc, err := protect(data)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append([]byte(encMagic), enc...), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
