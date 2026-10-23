package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"

	"redcyclone-vpn/internal/autostart"
	"redcyclone-vpn/internal/clash"
	"redcyclone-vpn/internal/runner"
	"redcyclone-vpn/internal/settings"
	"redcyclone-vpn/internal/singbox"
	"redcyclone-vpn/internal/tray"
)

// Connection states reported to the UI.
const (
	StateOff          = "off"
	StateOn           = "on"
	StateReconnecting = "reconnecting" // core died, retrying (kill switch may be blocking)
	StateBlocked      = "blocked"      // retries exhausted, kill switch keeps traffic blocked
)

// Status is pushed to the UI via the "status" event and returned by GetStatus.
type Status struct {
	Running  bool     `json:"running"` // anything of ours is up (state != off)
	State    string   `json:"state"`
	Mode     string   `json:"mode"`
	Since    string   `json:"since"`
	Admin    bool     `json:"admin"`
	BinsOK   bool     `json:"bins_ok"`
	Missing  []string `json:"missing"`
	Error    string   `json:"error"`
	Blocking bool     `json:"blocking"` // kill switch is currently rejecting traffic
	Pending  bool     `json:"pending"`  // saved settings differ from the running connection
	Server   string   `json:"server"`   // server in use (custom mode)
	Warnings []string `json:"warnings"`
}

// Stats is pushed every second via the "stats" event while connected.
type Stats struct {
	Up        int64 `json:"up"`   // bytes/s
	Down      int64 `json:"down"` // bytes/s
	UpTotal   int64 `json:"up_total"`
	DownTotal int64 `json:"down_total"`
}

var reconnectDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}

type App struct {
	ctx       context.Context
	autostart bool // launched by the logon task

	mu       sync.Mutex
	state    string
	sb       *runner.Proc
	dpi      *runner.Proc
	blocking bool              // sb runs the kill-switch config
	conf     settings.Settings // settings of the current connection
	since    time.Time
	lastErr  string
	warnings []string
	clash    *clash.Client
	server   string
	procGen  int // bumped whenever helpers start/stop; stale exit callbacks compare against it
	session  int // bumped on user Connect/Disconnect; cancels reconnect loops
	quitting bool

	setMu sync.Mutex // serialises settings read-modify-write

	logMu   sync.Mutex
	logFile *os.File
	logBuf  []string // recent lines, so the UI can show what happened before it loaded
}

const logKeep = 500

func NewApp(autostart bool) *App {
	return &App{state: StateOff, autostart: autostart}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Mirror the UI log to %APPDATA%\RedCyclone\app.log (fresh file each launch).
	if dir, err := settings.Dir(); err == nil {
		if f, err := os.Create(filepath.Join(dir, "app.log")); err == nil {
			a.logFile = f
		}
	}
	a.log(fmt.Sprintf("RedCyclone khởi động, admin=%v, bin=%s", isAdmin(), runner.BinDir()))
	if n, err := runner.KillStale(runner.BinDir()); n > 0 || err != nil {
		a.log(fmt.Sprintf("Dọn %d tiến trình cũ còn sót (err=%v)", n, err))
	}
	_ = runtime.InitializeNotifications(ctx)

	tray.Start(nil, tray.Callbacks{
		Show:   a.ShowWindow,
		Toggle: a.toggle,
		Quit:   a.Quit,
	})
	tray.SetState(false, "RedCyclone VPN: chưa kết nối")

	go a.statsLoop()

	if s, err := settings.Load(); err == nil {
		a.syncAutostart(s)
		if s.AutoConnect {
			go func() {
				if err := a.Connect(); err != nil {
					a.notify("Không tự kết nối được", err.Error())
				}
			}()
		}
	}
}

// beforeClose hides to the tray instead of quitting when enabled.
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if quitting {
		return false
	}
	if s, err := settings.Load(); err == nil && s.CloseToTray {
		runtime.WindowHide(ctx)
		return true
	}
	return false
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.Disconnect()
	tray.Stop()
}

// ShowWindow brings the window back from the tray / another instance.
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// Quit exits the app for real (tray menu / UI button).
func (a *App) Quit() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

func (a *App) toggle() {
	a.mu.Lock()
	on := a.state != StateOff
	a.mu.Unlock()
	if on {
		_ = a.Disconnect()
	} else if err := a.Connect(); err != nil {
		a.notify("Kết nối thất bại", err.Error())
	}
}

/* ---------------- Settings ---------------- */

func (a *App) GetSettings() (settings.Settings, error) {
	return settings.Load()
}

// SaveSettings persists s. While connected, a server switch is applied live;
// other network changes need a reconnect (Status.Pending tells the UI).
func (a *App) SaveSettings(s settings.Settings) error {
	a.setMu.Lock()
	defer a.setMu.Unlock()
	if err := settings.Save(s); err != nil {
		return err
	}
	a.syncAutostart(s)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state == StateOn && a.conf.Mode == settings.ModeCustom && a.conf.SelectedServer != s.SelectedServer && a.clash != nil {
		member := singbox.TagAuto
		if s.SelectedServer != settings.AutoServer {
			member = singbox.ServerTag(s.SelectedServer)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := a.clash.Select(ctx, singbox.TagProxy, member)
		cancel()
		if err == nil {
			a.conf.SelectedServer = s.SelectedServer
			a.log("Đã chuyển server.")
		} else {
			a.log("Không chuyển server trực tiếp được: " + err.Error())
		}
	}
	a.emitStatusLocked()
	return nil
}

// mutateSettings applies fn to the stored settings atomically.
func (a *App) mutateSettings(fn func(*settings.Settings) error) (settings.Settings, error) {
	a.setMu.Lock()
	defer a.setMu.Unlock()
	s, err := settings.Load()
	if err != nil {
		return s, err
	}
	if err := fn(&s); err != nil {
		return s, err
	}
	if err := settings.Save(s); err != nil {
		return s, err
	}
	a.mu.Lock()
	a.emitStatusLocked()
	a.mu.Unlock()
	return s, nil
}

func (a *App) syncAutostart(s settings.Settings) {
	if s.StartWithWindows == autostart.Enabled() {
		return
	}
	var err error
	if s.StartWithWindows {
		exe, e := os.Executable()
		if e != nil {
			err = e
		} else {
			err = autostart.Enable(exe, "--autostart")
		}
	} else {
		err = autostart.Disable()
	}
	if err != nil {
		a.log("Không cập nhật được khởi động cùng Windows: " + err.Error())
	}
}

// networkKey is the part of the settings that requires a reconnect to apply.
func networkKey(s settings.Settings) string {
	s.SelectedServer = ""
	s.AutoConnect, s.StartWithWindows, s.StartMinimized, s.CloseToTray = false, false, false, false
	s.KillSwitch, s.AutoReconnect = false, false
	s.Subscriptions = nil
	if s.Mode != settings.ModeCustom {
		s.Servers = nil
	}
	if s.Mode != settings.ModeWarp {
		s.Warp = nil
	}
	b, _ := json.Marshal(s)
	return string(b)
}

/* ---------------- Status ---------------- */

func (a *App) GetStatus() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	missing := runner.Missing(runner.BinDir())
	st := Status{
		Running:  a.state != StateOff,
		State:    a.state,
		Admin:    isAdmin(),
		BinsOK:   len(missing) == 0,
		Missing:  missing,
		Error:    a.lastErr,
		Blocking: a.blocking,
		Warnings: a.warnings,
	}
	if st.Running {
		st.Mode = string(a.conf.Mode)
		st.Since = a.since.Format(time.RFC3339)
		st.Server = a.server
		if saved, err := settings.Load(); err == nil {
			st.Pending = networkKey(saved) != networkKey(a.conf)
		}
	}
	return st
}

func (a *App) emitStatusLocked() {
	if a.ctx == nil {
		return
	}
	st := a.statusLocked()
	runtime.EventsEmit(a.ctx, "status", st)
	switch a.state {
	case StateOn:
		tray.SetState(true, "RedCyclone VPN: đã kết nối ("+modeName(a.conf.Mode)+")")
	case StateReconnecting:
		tray.SetState(false, "RedCyclone VPN: đang kết nối lại…")
	case StateBlocked:
		tray.SetState(false, "RedCyclone VPN: kill switch đang chặn mạng")
	default:
		tray.SetState(false, "RedCyclone VPN: chưa kết nối")
	}
}

/* ---------------- Connect / disconnect ---------------- */

func (a *App) Connect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != StateOff {
		return nil
	}
	a.session++
	a.lastErr = ""
	err := a.preflight()
	var s settings.Settings
	if err == nil {
		s, err = settings.Load()
	}
	if err == nil {
		err = a.startLocked(s)
	}
	if err != nil {
		a.lastErr = err.Error()
		a.log("Lỗi: " + err.Error())
	} else {
		a.state = StateOn
		a.since = time.Now()
		a.log("Đã kết nối.")
	}
	a.emitStatusLocked()
	return err
}

// Reconnect applies changed settings by restarting the connection.
func (a *App) Reconnect() error {
	if err := a.Disconnect(); err != nil {
		return err
	}
	return a.Connect()
}

func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session++ // cancels any reconnect loop
	if a.state == StateOff && a.sb == nil && a.dpi == nil {
		return nil
	}
	a.stopLocked()
	a.state = StateOff
	a.lastErr = ""
	a.warnings = nil
	a.log("Đã ngắt kết nối.")
	a.emitStatusLocked()
	return nil
}

func (a *App) preflight() error {
	if !isAdmin() {
		return errors.New("cần chạy ứng dụng với quyền Administrator")
	}
	bin := runner.BinDir()
	if m := runner.Missing(bin); len(m) > 0 {
		return errors.New("thiếu file trong " + bin + ": chạy scripts\\fetch-bins.ps1")
	}
	return nil
}

// startLocked starts sing-box (+ GoodbyeDPI) for s. On failure everything is stopped.
func (a *App) startLocked(s settings.Settings) error {
	bin := runner.BinDir()
	dir, err := settings.Dir()
	if err != nil {
		return err
	}
	addr, err := clash.FreeAddr()
	if err != nil {
		return err
	}
	secret := randomHex(16)
	cfg, warnings, err := singbox.Build(s, singbox.Options{
		BinDir:      bin,
		CacheFile:   filepath.Join(dir, "cache.db"),
		ClashAPI:    addr,
		ClashSecret: secret,
	})
	if err != nil {
		return err
	}
	for _, w := range warnings {
		a.log("Cảnh báo: " + w)
	}
	if err := a.runSingBox(cfg, "sing-box ("+string(s.Mode)+")"); err != nil {
		return err
	}
	a.blocking = false
	a.conf = s
	a.warnings = warnings
	a.clash = clash.New(addr, secret)
	a.server = ""

	if s.Mode == settings.ModeBypass && s.UseDPI {
		args := s.DPICommand()
		a.log(fmt.Sprintf("Khởi động GoodbyeDPI %v…", args))
		gen := a.procGen
		dpi, err := runner.Start("goodbyedpi", filepath.Join(bin, "goodbyedpi.exe"), args, a.log, a.onUnexpectedExit("goodbyedpi", gen))
		if err != nil {
			a.stopLocked()
			return err
		}
		a.dpi = dpi
		if exited, err := dpi.Wait(1500 * time.Millisecond); exited {
			a.stopLocked()
			return errors.New("GoodbyeDPI dừng ngay khi khởi động (xem log): " + errString(err))
		}
	}
	return nil
}

// startKillSwitchLocked replaces whatever runs with the traffic-blocking config.
func (a *App) startKillSwitchLocked() error {
	a.stopLocked()
	cfg, err := singbox.BuildKillSwitch(a.conf)
	if err != nil {
		return err
	}
	if err := a.runSingBox(cfg, "kill switch"); err != nil {
		return err
	}
	a.blocking = true
	return nil
}

// runSingBox writes cfg, starts sing-box and waits until it survives startup.
// The config file is deleted afterwards because it contains secrets.
func (a *App) runSingBox(cfg []byte, label string) error {
	dir, err := settings.Dir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		return err
	}
	defer os.Remove(cfgPath)

	a.procGen++
	gen := a.procGen
	a.log("Khởi động " + label + "…")
	sb, err := runner.Start("sing-box", filepath.Join(runner.BinDir(), "sing-box.exe"),
		[]string{"run", "-c", cfgPath, "-D", dir}, a.log, a.onUnexpectedExit("sing-box", gen))
	if err != nil {
		return err
	}
	// sing-box fails fast on bad config / TUN errors; give it a moment.
	if exited, err := sb.Wait(1500 * time.Millisecond); exited {
		return errors.New("sing-box dừng ngay khi khởi động (xem log): " + errString(err))
	}
	a.sb = sb
	return nil
}

func (a *App) stopLocked() {
	a.procGen++
	sb, dpi := a.sb, a.dpi
	a.sb, a.dpi = nil, nil
	a.clash = nil
	a.blocking = false
	dpi.Stop()
	sb.Stop()
}

// onUnexpectedExit handles a helper dying on its own while connected.
func (a *App) onUnexpectedExit(name string, gen int) func(error) {
	return func(err error) {
		go func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			if gen != a.procGen || a.state == StateOff {
				return // stopped on purpose, or a newer process is in charge
			}
			msg := name + " đã dừng bất ngờ: " + errString(err)
			a.log(msg)
			a.lastErr = msg
			a.handleFailureLocked()
			a.emitStatusLocked()
		}()
	}
}

// handleFailureLocked decides what to do after the tunnel died.
func (a *App) handleFailureLocked() {
	protect := a.conf.KillSwitch && a.conf.Mode != settings.ModeBypass
	if protect {
		if err := a.startKillSwitchLocked(); err != nil {
			a.log("Không bật được kill switch: " + err.Error())
		} else {
			a.log("Kill switch: đang chặn mọi kết nối để không lộ IP.")
		}
	} else {
		a.stopLocked()
	}
	if a.conf.AutoReconnect {
		a.state = StateReconnecting
		go a.reconnectLoop(a.session)
		return
	}
	if protect && a.blocking {
		a.state = StateBlocked
		a.notify("Mất kết nối VPN", "Kill switch đang chặn mạng. Mở RedCyclone để kết nối lại hoặc ngắt.")
		return
	}
	a.state = StateOff
	a.notify("Mất kết nối VPN", a.lastErr)
}

func (a *App) reconnectLoop(session int) {
	for i, d := range reconnectDelays {
		time.Sleep(d)
		a.mu.Lock()
		if a.session != session {
			a.mu.Unlock()
			return
		}
		a.log(fmt.Sprintf("Kết nối lại (lần %d/%d)…", i+1, len(reconnectDelays)))
		conf := a.conf
		a.stopLocked()
		err := a.startLocked(conf)
		if err == nil {
			a.state = StateOn
			a.lastErr = ""
			a.since = time.Now()
			a.log("Đã kết nối lại.")
			a.emitStatusLocked()
			a.mu.Unlock()
			a.notify("Đã kết nối lại", "RedCyclone VPN hoạt động trở lại.")
			return
		}
		a.lastErr = err.Error()
		a.log("Kết nối lại thất bại: " + err.Error())
		if conf.KillSwitch && conf.Mode != settings.ModeBypass {
			a.conf = conf
			_ = a.startKillSwitchLocked()
		}
		a.emitStatusLocked()
		a.mu.Unlock()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.session != session {
		return
	}
	if a.blocking {
		a.state = StateBlocked
		a.notify("Không kết nối lại được", "Kill switch vẫn đang chặn mạng. Mở RedCyclone để thử lại hoặc ngắt.")
	} else {
		a.stopLocked()
		a.state = StateOff
		a.notify("Không kết nối lại được", a.lastErr)
	}
	a.emitStatusLocked()
}

/* ---------------- Stats ---------------- */

func (a *App) statsLoop() {
	var lastUp, lastDown int64
	var lastClient *clash.Client
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for range tick.C {
		a.mu.Lock()
		c, on, mode := a.clash, a.state == StateOn, a.conf.Mode
		a.mu.Unlock()
		if !on || c == nil {
			lastClient = nil
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
		up, down, err := c.Totals(ctx)
		server := ""
		if err == nil && mode == settings.ModeCustom {
			server = a.currentServer(ctx, c)
		}
		cancel()
		if err != nil {
			continue
		}
		st := Stats{UpTotal: up, DownTotal: down}
		if c == lastClient {
			st.Up, st.Down = max(0, up-lastUp), max(0, down-lastDown)
		}
		lastClient, lastUp, lastDown = c, up, down
		runtime.EventsEmit(a.ctx, "stats", st)

		a.mu.Lock()
		if a.clash == c && a.server != server {
			a.server = server
			a.emitStatusLocked()
		}
		a.mu.Unlock()
	}
}

// currentServer resolves selector → (auto →) server tag → display name.
func (a *App) currentServer(ctx context.Context, c *clash.Client) string {
	tag, err := c.Now(ctx, singbox.TagProxy)
	if err != nil {
		return ""
	}
	auto := tag == singbox.TagAuto
	if auto {
		if tag, err = c.Now(ctx, singbox.TagAuto); err != nil {
			return "Tự động"
		}
	}
	a.mu.Lock()
	servers := a.conf.Servers
	a.mu.Unlock()
	for _, s := range servers {
		if singbox.ServerTag(s.ID) == tag {
			if auto {
				return "Tự động: " + s.Name
			}
			return s.Name
		}
	}
	return tag
}

/* ---------------- Misc ---------------- */

// OpenDataFolder opens %APPDATA%\RedCyclone (logs, settings) in Explorer.
func (a *App) OpenDataFolder() error {
	dir, err := settings.Dir()
	if err != nil {
		return err
	}
	return exec.Command("explorer.exe", dir).Start()
}

func (a *App) notify(title, body string) {
	if a.ctx == nil {
		return
	}
	_ = runtime.SendNotification(a.ctx, runtime.NotificationOptions{
		ID:    "rcv-" + randomHex(4),
		Title: title,
		Body:  body,
	})
}

// GetLogs returns the recent log lines (the UI loads them once on start).
func (a *App) GetLogs() []string {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	return append([]string(nil), a.logBuf...)
}

func (a *App) log(line string) {
	a.logMu.Lock()
	if a.logFile != nil {
		fmt.Fprintf(a.logFile, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
	}
	a.logBuf = append(a.logBuf, line)
	if len(a.logBuf) > logKeep {
		a.logBuf = a.logBuf[len(a.logBuf)-logKeep:]
	}
	a.logMu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log", line)
	}
}

func modeName(m settings.Mode) string {
	switch m {
	case settings.ModeWarp:
		return "WARP"
	case settings.ModeCustom:
		return "server riêng"
	}
	return "vượt chặn"
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func errString(err error) string {
	if err == nil {
		return "exit 0"
	}
	return err.Error()
}
