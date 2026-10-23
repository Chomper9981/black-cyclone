package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"

	"redcyclone-vpn/internal/runner"
	"redcyclone-vpn/internal/settings"
	"redcyclone-vpn/internal/singbox"
	"redcyclone-vpn/internal/warp"
)

// Status is pushed to the UI via the "status" event and returned by GetStatus.
type Status struct {
	Running bool     `json:"running"`
	Mode    string   `json:"mode"`
	Since   string   `json:"since"`
	Admin   bool     `json:"admin"`
	BinsOK  bool     `json:"bins_ok"`
	Missing []string `json:"missing"`
	Error   string   `json:"error"`
}

type App struct {
	ctx context.Context

	mu      sync.Mutex
	sb      *runner.Proc
	dpi     *runner.Proc
	mode    settings.Mode
	since   time.Time
	lastErr string

	logMu   sync.Mutex
	logFile *os.File
}

func NewApp() *App {
	return &App{}
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
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.Disconnect()
}

func (a *App) GetSettings() (settings.Settings, error) {
	return settings.Load()
}

func (a *App) SaveSettings(s settings.Settings) error {
	a.mu.Lock()
	running := a.sb != nil
	a.mu.Unlock()
	if running {
		return errors.New("hãy ngắt kết nối trước khi đổi cài đặt")
	}
	return settings.Save(s)
}

func (a *App) RegisterWarp() (settings.Settings, error) {
	s, err := settings.Load()
	if err != nil {
		return s, err
	}
	a.log("Đang đăng ký tài khoản Cloudflare WARP…")
	w, err := warp.Register()
	if err != nil {
		a.log("Đăng ký WARP thất bại: " + err.Error())
		return s, err
	}
	s.Warp = w
	if err := settings.Save(s); err != nil {
		return s, err
	}
	a.log("Đăng ký WARP thành công.")
	return s, nil
}

func (a *App) GetStatus() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	missing := runner.Missing(runner.BinDir())
	st := Status{
		Running: a.sb != nil,
		Admin:   isAdmin(),
		BinsOK:  len(missing) == 0,
		Missing: missing,
		Error:   a.lastErr,
	}
	if st.Running {
		st.Mode = string(a.mode)
		st.Since = a.since.Format(time.RFC3339)
	}
	return st
}

func (a *App) Connect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sb != nil {
		return nil
	}
	err := a.connectLocked()
	if err != nil {
		a.lastErr = err.Error()
		a.log("Lỗi: " + err.Error())
	}
	a.emitStatusLocked()
	return err
}

func (a *App) connectLocked() error {
	a.lastErr = ""
	if !isAdmin() {
		return errors.New("cần chạy ứng dụng với quyền Administrator")
	}
	bin := runner.BinDir()
	if m := runner.Missing(bin); len(m) > 0 {
		return errors.New("thiếu file trong " + bin + ": chạy scripts\\fetch-bins.ps1")
	}
	s, err := settings.Load()
	if err != nil {
		return err
	}
	cfg, err := singbox.Build(s)
	if err != nil {
		return err
	}
	dir, err := settings.Dir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		return err
	}

	a.log("Khởi động sing-box (" + string(s.Mode) + ")…")
	sb, err := runner.Start("sing-box", filepath.Join(bin, "sing-box.exe"),
		[]string{"run", "-c", cfgPath, "-D", dir}, a.log, a.onUnexpectedExit("sing-box"))
	if err != nil {
		return err
	}
	// sing-box fails fast on bad config / TUN errors; give it a moment.
	if exited, err := sb.Wait(1500 * time.Millisecond); exited {
		return errors.New("sing-box dừng ngay khi khởi động (xem log): " + errString(err))
	}
	a.sb = sb

	if s.Mode == settings.ModeBypass && s.UseDPI {
		a.log("Khởi động GoodbyeDPI…")
		// -5: GoodbyeDPI 0.2.x default modeset (-f 2 -e 2 --auto-ttl --reverse-frag --max-payload).
		dpi, err := runner.Start("goodbyedpi", filepath.Join(bin, "goodbyedpi.exe"),
			[]string{"-5"}, a.log, a.onUnexpectedExit("goodbyedpi"))
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

	a.mode = s.Mode
	a.since = time.Now()
	a.log("Đã kết nối.")
	return nil
}

// onUnexpectedExit tears everything down if a helper dies while connected.
func (a *App) onUnexpectedExit(name string) func(error) {
	return func(err error) {
		go func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.sb == nil {
				return // stopped on purpose
			}
			a.lastErr = name + " đã dừng bất ngờ: " + errString(err)
			a.log(a.lastErr)
			a.stopLocked()
			a.emitStatusLocked()
		}()
	}
}

func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sb == nil && a.dpi == nil {
		return nil
	}
	a.stopLocked()
	a.lastErr = ""
	a.log("Đã ngắt kết nối.")
	a.emitStatusLocked()
	return nil
}

func (a *App) stopLocked() {
	sb, dpi := a.sb, a.dpi
	a.sb, a.dpi = nil, nil
	dpi.Stop()
	sb.Stop()
}

func (a *App) emitStatusLocked() {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "status", a.statusLocked())
	}
}

func (a *App) log(line string) {
	a.logMu.Lock()
	if a.logFile != nil {
		fmt.Fprintf(a.logFile, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
	}
	a.logMu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log", line)
	}
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
