package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"black-cyclone/internal/settings"
	"black-cyclone/internal/vps"
)

// The server script ships inside the app, so "Cài đặt / cập nhật" always
// installs the version that matches this build.
//
//go:embed server/bcserver.sh
var bcserverRaw []byte

var bcserverScript, bcserverSum = vps.Script(bcserverRaw)

// vpsMu serialises access to vps.json and to each VPS (one operation at a time).
var (
	vpsMu    sync.Mutex
	vpsBusy  = map[string]*sync.Mutex{}
	vpsBusyM sync.Mutex
)

func hostLock(id string) *sync.Mutex {
	vpsBusyM.Lock()
	defer vpsBusyM.Unlock()
	m := vpsBusy[id]
	if m == nil {
		m = &sync.Mutex{}
		vpsBusy[id] = m
	}
	return m
}

// VPSView is what the UI sees of a Host: never the password.
type VPSView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Addr        string `json:"addr"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Auth        string `json:"auth"` // "password" | "key"
	KeyFile     string `json:"key_file"`
	Fingerprint string `json:"fingerprint"`
}

func viewOf(h vps.Host) VPSView {
	v := VPSView{ID: h.ID, Name: h.Name, Addr: h.Addr, Port: h.Port, User: h.User, KeyFile: h.KeyFile,
		Fingerprint: vps.Fingerprint(h.HostKey), Auth: "password"}
	if h.KeyFile != "" {
		v.Auth = "key"
	}
	return v
}

// VPSInput is the "add VPS" form.
type VPSInput struct {
	Name     string `json:"name"`
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	KeyFile  string `json:"key_file"`
}

func (a *App) ListVPS() ([]VPSView, error) {
	vpsMu.Lock()
	defer vpsMu.Unlock()
	hosts, err := vps.Load()
	out := make([]VPSView, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, viewOf(h))
	}
	return out, err
}

// AddVPS checks the login (and root rights) before saving, and pins the SSH host key.
func (a *App) AddVPS(in VPSInput) (VPSView, error) {
	h := vps.Host{
		ID:       settings.NewID(),
		Name:     strings.TrimSpace(in.Name),
		Addr:     strings.TrimSpace(in.Addr),
		Port:     in.Port,
		User:     strings.TrimSpace(in.User),
		Password: in.Password,
		KeyFile:  strings.TrimSpace(in.KeyFile),
	}
	if h.Port == 0 {
		h.Port = 22
	}
	if h.User == "" {
		h.User = "root"
	}
	h.Addr = strings.Trim(h.Addr, "[]")
	if h.Addr == "" || strings.ContainsAny(h.Addr, " /@") {
		return VPSView{}, errors.New("nhập IP hoặc tên miền của VPS")
	}
	if h.Port < 1 || h.Port > 65535 {
		return VPSView{}, errors.New("cổng SSH phải từ 1 đến 65535")
	}
	if h.Name == "" {
		h.Name = h.Addr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	a.log("Đang kiểm tra đăng nhập SSH vào " + h.Addr + "…")
	c, err := vps.Dial(ctx, &h)
	if err != nil {
		a.log("SSH thất bại: " + err.Error())
		return VPSView{}, err
	}
	defer c.Close()
	if err := c.CheckRoot(ctx); err != nil {
		return VPSView{}, err
	}

	vpsMu.Lock()
	defer vpsMu.Unlock()
	hosts, err := vps.Load()
	if err != nil {
		return VPSView{}, err
	}
	hosts = append(hosts, h)
	if err := vps.Save(hosts); err != nil {
		return VPSView{}, err
	}
	a.log(fmt.Sprintf("Đã thêm VPS %s (khoá SSH %s).", h.Name, vps.Fingerprint(h.HostKey)))
	return viewOf(h), nil
}

// RemoveVPS forgets a VPS in this app; the server on it keeps running.
func (a *App) RemoveVPS(id string) error {
	vpsMu.Lock()
	defer vpsMu.Unlock()
	hosts, err := vps.Load()
	if err != nil {
		return err
	}
	out := hosts[:0]
	for _, h := range hosts {
		if h.ID != id {
			out = append(out, h)
		}
	}
	return vps.Save(out)
}

// PickKeyFile opens a file dialog for an SSH private key.
func (a *App) PickKeyFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Chọn file khoá SSH (private key)"})
}

// withVPS connects to host id and runs fn, one operation per VPS at a time.
func (a *App) withVPS(id string, timeout time.Duration, fn func(ctx context.Context, c *vps.Client) error) error {
	vpsMu.Lock()
	hosts, err := vps.Load()
	vpsMu.Unlock()
	if err != nil {
		return err
	}
	var h *vps.Host
	for i := range hosts {
		if hosts[i].ID == id {
			h = &hosts[i]
		}
	}
	if h == nil {
		return errors.New("không tìm thấy VPS")
	}
	if h.HostKey == "" {
		return errors.New("VPS chưa có khoá SSH đã xác minh: xoá rồi thêm lại")
	}
	l := hostLock(id)
	if !l.TryLock() {
		return errors.New("VPS đang bận với thao tác khác, đợi một chút")
	}
	defer l.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := vps.Dial(ctx, h)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(ctx, c)
}

func (a *App) VPSOverview(id string) (o vps.Overview, err error) {
	err = a.withVPS(id, 45*time.Second, func(ctx context.Context, c *vps.Client) error {
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

// VPSInstall installs or updates the server; output streams on "vps-out".
func (a *App) VPSInstall(id string, opts vps.InstallOptions) (o vps.Overview, err error) {
	emit := func(line string) { runtime.EventsEmit(a.ctx, "vps-out", map[string]string{"id": id, "line": line}) }
	err = a.withVPS(id, 10*time.Minute, func(ctx context.Context, c *vps.Client) error {
		a.log("Đang cài / cập nhật server trên VPS…")
		if err := c.Install(ctx, bcserverScript, opts, emit); err != nil {
			a.log("Cài server thất bại: " + err.Error())
			return err
		}
		a.log("Cài server xong.")
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

func (a *App) VPSAddUser(id, name string) (o vps.Overview, err error) {
	name = strings.TrimSpace(name)
	err = a.withVPS(id, time.Minute, func(ctx context.Context, c *vps.Client) error {
		if _, err := c.AddUser(ctx, name); err != nil {
			return err
		}
		a.log("Đã thêm người dùng " + name + " trên VPS.")
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

func (a *App) VPSRemoveUser(id, name string) (o vps.Overview, err error) {
	err = a.withVPS(id, time.Minute, func(ctx context.Context, c *vps.Client) error {
		if err := c.RemoveUser(ctx, name); err != nil {
			return err
		}
		a.log("Đã xoá người dùng " + name + " trên VPS.")
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

func (a *App) VPSRestart(id string) (o vps.Overview, err error) {
	err = a.withVPS(id, time.Minute, func(ctx context.Context, c *vps.Client) error {
		if err := c.Restart(ctx); err != nil {
			return err
		}
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

func (a *App) VPSLogs(id string) (out string, err error) {
	err = a.withVPS(id, time.Minute, func(ctx context.Context, c *vps.Client) error {
		out, err = c.Logs(ctx)
		return err
	})
	return out, err
}

// VPSUninstall removes the server from the VPS (all users' links stop working).
func (a *App) VPSUninstall(id string) (o vps.Overview, err error) {
	err = a.withVPS(id, 2*time.Minute, func(ctx context.Context, c *vps.Client) error {
		if err := c.Uninstall(ctx); err != nil {
			return err
		}
		a.log("Đã gỡ server khỏi VPS.")
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}
