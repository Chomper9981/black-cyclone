// Package rcvcore is the gomobile-bound API used by the Android app (Java class
// rcvcore.Rcvcore). It runs sing-box in-process on the TUN that the app's
// VpnService establishes, with the same configuration builder as the desktop
// app (internal/singbox). Only gomobile-friendly types cross the boundary:
// structured values travel as JSON strings.
package rcvcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"black-cyclone/internal/clash"
	"black-cyclone/internal/settings"
	"black-cyclone/internal/share"
	"black-cyclone/internal/singbox"
	"black-cyclone/internal/warp"
)

const probeURL = "https://www.gstatic.com/generate_204"

var (
	mu  sync.Mutex
	cur *session

	scanMu     sync.Mutex
	scanCancel context.CancelFunc
)

func parseAccount(accountJSON string) (*settings.Warp, error) {
	var a settings.Warp
	if err := json.Unmarshal([]byte(accountJSON), &a); err != nil {
		return nil, fmt.Errorf("tài khoản WARP hỏng: %w", err)
	}
	return &a, nil
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

/* ---------------- WARP account ---------------- */

// Register creates a new free WARP device and returns it as account JSON.
func Register() (string, error) {
	a, err := warp.Register()
	if err != nil {
		return "", err
	}
	logf("Đã đăng ký WARP mới (%s)", a.AccountType)
	return toJSON(a), nil
}

// ApplyLicense binds a WARP+ key to the account and returns the updated account JSON.
func ApplyLicense(accountJSON, license string) (string, error) {
	a, err := parseAccount(accountJSON)
	if err != nil {
		return "", err
	}
	if err := warp.ApplyLicense(a, license); err != nil {
		return "", err
	}
	logf("Đã áp dụng key WARP+ (%s)", a.AccountType)
	return toJSON(a), nil
}

type scanResult struct {
	IP    string `json:"ip"`
	Port  int    `json:"port"`
	RTTMs int64  `json:"rtt_ms"`
}

type scanReport struct {
	Scanned int          `json:"scanned"`
	OK      []scanResult `json:"ok"` // fastest first, at most 20
}

// Scan probes random WARP endpoints with real WireGuard handshakes for up to
// timeoutSec seconds and returns a scanReport JSON. CancelScan stops it early.
func Scan(accountJSON string, parallel, timeoutSec int) (string, error) {
	a, err := parseAccount(accountJSON)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
	}
	scanCancel = cancel
	scanMu.Unlock()

	cands := warp.DefaultCandidates()
	res := warp.Scan(ctx, a, cands, parallel)
	rep := scanReport{Scanned: len(cands), OK: []scanResult{}}
	ok := 0
	for _, r := range res {
		if !r.OK {
			continue
		}
		ok++
		if len(rep.OK) < 20 {
			rep.OK = append(rep.OK, scanResult{r.IP, r.Port, r.RTT.Milliseconds()})
		}
	}
	logf("Quét %d endpoint: %d phản hồi", len(cands), ok)
	return toJSON(rep), nil
}

// CancelScan stops a running Scan; it returns what it has found so far.
func CancelScan() {
	scanMu.Lock()
	defer scanMu.Unlock()
	if scanCancel != nil {
		scanCancel()
		scanCancel = nil
	}
}

/* ---------------- Servers ---------------- */

type linkInfo struct {
	Name  string `json:"name"`
	Link  string `json:"link"`
	Error string `json:"error,omitempty"`
}

// ParseLinks finds every supported share link in text (vless, vmess, trojan,
// ss, hysteria2, tuic) and returns them as a JSON array of linkInfo; links the
// parser rejects carry an error.
func ParseLinks(text string) string {
	out := []linkInfo{}
	for _, l := range share.ParseLinks(text) {
		li := linkInfo{Name: share.Name(l), Link: l}
		if _, err := share.ParseOutbound(l, "x"); err != nil {
			li.Error = err.Error()
		}
		out = append(out, li)
	}
	return toJSON(out)
}

// NewID returns a short random identifier for a server.
func NewID() string { return settings.NewID() }

/* ---------------- Tunnel ---------------- */

type session struct {
	box    *box.Box
	ctx    context.Context
	cancel context.CancelFunc
	tunFd  int
	clash  *clash.Client
	conf   settings.Settings

	mu       sync.Mutex
	lastOK   time.Time
	delay    time.Duration
	probeErr string
}

// Start runs sing-box for the settings (JSON of the desktop app's
// settings.Settings) on the TUN fd from
// VpnService.Builder.establish().detachFd(). The fd belongs to the core from
// here on, also on error. dataDir holds the cache and, if present,
// geoip-vn.srs.
func Start(fd int, settingsJSON, dataDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if cur != nil {
		cur.close()
		cur = nil
	}
	s, err := start(fd, settingsJSON, dataDir)
	if err != nil {
		closeFd(fd)
		logf("Không khởi động được: %v", err)
		return err
	}
	cur = s
	logf("Đã bật %s", modeName(s.conf))
	go s.probeLoop()
	return nil
}

func modeName(s settings.Settings) string {
	if s.Mode == settings.ModeCustom {
		return "Server riêng"
	}
	return "WARP"
}

func start(fd int, settingsJSON, dataDir string) (*session, error) {
	platformMu.Lock()
	p := platformIf
	platformMu.Unlock()
	if p == nil {
		return nil, errors.New("chưa gọi SetPlatform")
	}
	conf := settings.Default()
	if err := json.Unmarshal([]byte(settingsJSON), &conf); err != nil {
		return nil, fmt.Errorf("cài đặt hỏng: %w", err)
	}
	// Per-app rules are applied by the VpnService; process rules don't exist here.
	conf.Routing.DirectProcesses = nil

	addr, err := clash.FreeAddr()
	if err != nil {
		return nil, err
	}
	secret := randomHex(16)
	cfg, warnings, err := singbox.Build(conf, singbox.Options{
		BinDir:      dataDir,
		CacheFile:   filepath.Join(dataDir, "cache.db"),
		ClashAPI:    addr,
		ClashSecret: secret,
	})
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		logf("%s", w)
	}

	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	opts, err := sjson.UnmarshalExtendedContext[option.Options](ctx, cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("config sing-box: %w", err)
	}
	opts.Log = &option.LogOptions{Level: "info"} // the app adds its own timestamps
	ctx = service.ContextWith[adapter.PlatformInterface](ctx, &platform{p: p, tunFd: fd})
	instance, err := box.New(box.Options{Context: ctx, Options: opts, PlatformLogWriter: logWriter{}})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("tạo sing-box: %w", err)
	}
	if err := instance.Start(); err != nil {
		instance.Close()
		cancel()
		return nil, fmt.Errorf("chạy sing-box: %w", err)
	}
	return &session{
		box: instance, ctx: ctx, cancel: cancel, tunFd: fd,
		clash: clash.New(addr, secret), conf: conf,
	}, nil
}

func (s *session) close() {
	s.cancel()
	s.box.Close()
	closeFd(s.tunFd)
}

// probeLoop checks the tunnel with a real HTTPS request through it, soon after
// start and then every 20s, so the app can tell "connected" from "stuck".
func (s *session) probeLoop() {
	wait := 500 * time.Millisecond
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(wait):
		}
		s.probe()
		wait = 20 * time.Second
		s.mu.Lock()
		if s.lastOK.IsZero() {
			wait = 3 * time.Second // keep trying quickly until the first success
		}
		s.mu.Unlock()
	}
}

func (s *session) probe() {
	var detour N.Dialer
	if ep, ok := s.box.Endpoint().Get(singbox.TagProxy); ok {
		detour = ep
	} else if ob, ok := s.box.Outbound().Outbound(singbox.TagProxy); ok {
		detour = ob
	} else {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	defer cancel()
	ms, err := urltest.URLTest(ctx, probeURL, detour)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if s.ctx.Err() == nil && s.probeErr != err.Error() {
			logf("Kiểm tra kết nối: %v", err)
		}
		s.probeErr = err.Error()
		return
	}
	if s.lastOK.IsZero() {
		logf("Đã thông: %d ms", ms)
	}
	s.lastOK, s.delay, s.probeErr = time.Now(), time.Duration(ms)*time.Millisecond, ""
}

// Stop shuts the tunnel down.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if cur != nil {
		cur.close()
		cur = nil
		logf("Đã tắt")
	}
}

type statsJSON struct {
	Running bool  `json:"running"`
	Up      int64 `json:"up"`
	Down    int64 `json:"down"`
	// Milliseconds since the last successful check through the tunnel, -1 if none yet.
	OKAgoMs int64  `json:"ok_ago_ms"`
	DelayMs int64  `json:"delay_ms"`
	Error   string `json:"error,omitempty"`
	Server  string `json:"server,omitempty"` // current server's display name (Server riêng)
}

// Stats returns traffic counters and tunnel health as JSON.
func Stats() string {
	mu.Lock()
	s := cur
	mu.Unlock()
	out := statsJSON{OKAgoMs: -1}
	if s == nil {
		return toJSON(out)
	}
	out.Running = true
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	if up, down, err := s.clash.Totals(ctx); err == nil {
		out.Up, out.Down = up, down
	}
	if s.conf.Mode == settings.ModeCustom {
		out.Server = s.currentServer(ctx)
	}
	s.mu.Lock()
	if !s.lastOK.IsZero() {
		out.OKAgoMs = time.Since(s.lastOK).Milliseconds()
		out.DelayMs = s.delay.Milliseconds()
	}
	out.Error = s.probeErr
	s.mu.Unlock()
	return toJSON(out)
}

// currentServer resolves selector → (auto →) server tag → display name.
func (s *session) currentServer(ctx context.Context) string {
	tag, err := s.clash.Now(ctx, singbox.TagProxy)
	if err != nil {
		return ""
	}
	auto := tag == singbox.TagAuto
	if auto {
		if tag, err = s.clash.Now(ctx, singbox.TagAuto); err != nil {
			return "Tự động"
		}
	}
	for _, srv := range s.conf.Servers {
		if singbox.ServerTag(srv.ID) == tag {
			if auto {
				return "Tự động: " + srv.Name
			}
			return srv.Name
		}
	}
	return tag
}

// SelectServer switches the running tunnel to a server ID (or "auto") without
// reconnecting. It fails when not connected in Server riêng mode, or when the
// server is not part of the running configuration.
func SelectServer(id string) error {
	mu.Lock()
	s := cur
	mu.Unlock()
	if s == nil || s.conf.Mode != settings.ModeCustom {
		return errors.New("chưa kết nối Server riêng")
	}
	member := singbox.TagAuto
	if id != settings.AutoServer {
		member = singbox.ServerTag(id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.clash.Select(ctx, singbox.TagProxy, member); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastOK = time.Time{}
	s.mu.Unlock()
	go s.probe()
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// logWriter forwards sing-box's log to the app's log, dropping debug noise.
type logWriter struct{}

// ansiColor matches the colour codes sing-box always puts in platform log
// lines (its LogOptions.DisableColor does not apply to them).
var ansiColor = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// logNoise are per-connection / per-query lines that would flood the
// 400-line buffer; "closed" lines are routine when the network changes.
var logNoise = []string{
	"inbound connection", "outbound connection", "inbound packet", "inbound DNS packet",
	"search process", "found user id", "found package",
	"dns: cached", "dns: exchanged", "lookup succeed",
	"connection download closed", "connection upload closed",
}

func (logWriter) WriteMessage(level log.Level, message string) {
	if level > log.LevelInfo {
		return
	}
	for _, noise := range logNoise {
		if strings.Contains(message, noise) {
			return
		}
	}
	logf("%s", ansiColor.ReplaceAllString(message, ""))
}
