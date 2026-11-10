package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"black-cyclone/internal/i18n"
	"black-cyclone/internal/settings"
	"black-cyclone/internal/vps"
	"black-cyclone/server"
)

// The server script ships inside the app, so "Cài đặt / cập nhật" always
// installs the version that matches this build.
var bcserverScript, bcserverSum = vps.Script(server.Script)

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

// hostFromInput validates the form into a Host (no ID, secrets as given).
func hostFromInput(in VPSInput) (vps.Host, error) {
	h := vps.Host{
		Name:     strings.TrimSpace(in.Name),
		Addr:     strings.Trim(strings.TrimSpace(in.Addr), "[]"),
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
	if h.Addr == "" || strings.ContainsAny(h.Addr, " /@") {
		return h, errors.New(i18n.T("nhập IP hoặc tên miền của VPS", "enter the VPS's IP or domain"))
	}
	if h.Port < 1 || h.Port > 65535 {
		return h, errors.New(i18n.T("cổng SSH phải từ 1 đến 65535", "the SSH port must be between 1 and 65535"))
	}
	if h.Name == "" {
		h.Name = h.Addr
	}
	return h, nil
}

// verifyLogin logs in (pinning the host key if h has none) and checks root rights.
func (a *App) verifyLogin(h *vps.Host) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	a.log(i18n.T("Đang kiểm tra đăng nhập SSH vào ", "Checking the SSH login to ") + h.Addr + "…")
	c, err := vps.Dial(ctx, h)
	if err != nil {
		a.log(i18n.T("SSH thất bại: ", "SSH failed: ") + err.Error())
		return err
	}
	defer c.Close()
	return c.CheckRoot(ctx)
}

// saveHost replaces the host with the same ID, or appends it.
func saveHost(h vps.Host) error {
	vpsMu.Lock()
	defer vpsMu.Unlock()
	hosts, err := vps.Load()
	if err != nil {
		return err
	}
	for i := range hosts {
		if hosts[i].ID == h.ID {
			hosts[i] = h
			return vps.Save(hosts)
		}
	}
	return vps.Save(append(hosts, h))
}

func loadHost(id string) (vps.Host, error) {
	vpsMu.Lock()
	defer vpsMu.Unlock()
	hosts, err := vps.Load()
	if err != nil {
		return vps.Host{}, err
	}
	for _, h := range hosts {
		if h.ID == id {
			return h, nil
		}
	}
	return vps.Host{}, errors.New(i18n.T("không tìm thấy VPS", "VPS not found"))
}

// AddVPS checks the login (and root rights) before saving, and pins the SSH host key.
func (a *App) AddVPS(in VPSInput) (VPSView, error) {
	h, err := hostFromInput(in)
	if err != nil {
		return VPSView{}, err
	}
	h.ID = settings.NewID()
	if err := a.verifyLogin(&h); err != nil {
		return VPSView{}, err
	}
	if err := saveHost(h); err != nil {
		return VPSView{}, err
	}
	a.log(fmt.Sprintf(i18n.T("Đã thêm VPS %s (khoá SSH %s).", "Added VPS %s (SSH key %s)."), h.Name, vps.Fingerprint(h.HostKey)))
	return viewOf(h), nil
}

// UpdateVPS edits a saved VPS. An empty password keeps the saved one. The
// login is checked again unless only the name changed; the pinned host key is
// kept (and enforced) while the address and port stay the same.
func (a *App) UpdateVPS(id string, in VPSInput) (VPSView, error) {
	old, err := loadHost(id)
	if err != nil {
		return VPSView{}, err
	}
	h, err := hostFromInput(in)
	if err != nil {
		return VPSView{}, err
	}
	h.ID = id
	if h.Password == "" {
		h.Password = old.Password
	}
	if h.Addr == old.Addr && h.Port == old.Port {
		h.HostKey = old.HostKey
	}
	same := h.Addr == old.Addr && h.Port == old.Port && h.User == old.User &&
		h.Password == old.Password && h.KeyFile == old.KeyFile
	if !same {
		if err := a.verifyLogin(&h); err != nil {
			return VPSView{}, err
		}
	}
	if err := saveHost(h); err != nil {
		return VPSView{}, err
	}
	a.log(i18n.T("Đã lưu thay đổi VPS ", "Saved changes to VPS ") + h.Name + ".")
	return viewOf(h), nil
}

// TrustNewVPSKey accepts the VPS's current SSH key after the user confirmed
// that the VPS was reinstalled (the old key no longer matches).
func (a *App) TrustNewVPSKey(id string) (VPSView, error) {
	h, err := loadHost(id)
	if err != nil {
		return VPSView{}, err
	}
	old := vps.Fingerprint(h.HostKey)
	h.HostKey = ""
	if err := a.verifyLogin(&h); err != nil {
		return VPSView{}, err
	}
	if err := saveHost(h); err != nil {
		return VPSView{}, err
	}
	a.log(fmt.Sprintf(i18n.T("VPS %s: đã chấp nhận khoá SSH mới %s (khoá cũ %s).", "VPS %s: accepted the new SSH key %s (old key %s)."), h.Name, vps.Fingerprint(h.HostKey), old))
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
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: i18n.T("Chọn file khoá SSH (private key)", "Choose the SSH private key file")})
}

// withVPS connects to host id and runs fn, one operation per VPS at a time.
func (a *App) withVPS(id string, timeout time.Duration, fn func(ctx context.Context, c *vps.Client) error) error {
	host, err := loadHost(id)
	if err != nil {
		return err
	}
	h := &host
	if h.HostKey == "" {
		return errors.New(i18n.T("VPS chưa có khoá SSH đã xác minh: xoá rồi thêm lại", "this VPS has no verified SSH key: remove it and add it again"))
	}
	// Wait for the operation in progress (e.g. the UI's periodic refresh).
	l := hostLock(id)
	l.Lock()
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
		a.log(i18n.T("Đang cài / cập nhật server trên VPS…", "Installing / updating the server on the VPS…"))
		if err := c.Install(ctx, bcserverScript, opts, emit); err != nil {
			a.log(i18n.T("Cài server thất bại: ", "Server install failed: ") + err.Error())
			return err
		}
		a.log(i18n.T("Cài server xong.", "Server installed."))
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
		a.log(i18n.T("Đã thêm người dùng ", "Added user ") + name + i18n.T(" trên VPS.", " on the VPS."))
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
		a.log(i18n.T("Đã xoá người dùng ", "Removed user ") + name + i18n.T(" trên VPS.", " on the VPS."))
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
		a.log(i18n.T("Đã gỡ server khỏi VPS.", "Server removed from the VPS."))
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}

// VPSLinkQR renders a share link as a QR code (PNG data URL) so a phone can
// scan it instead of receiving the link through a chat app.
func (a *App) VPSLinkQR(link string) (string, error) {
	if !strings.HasPrefix(link, "vless://") || len(link) > 2048 {
		return "", errors.New(i18n.T("link không hợp lệ", "invalid link"))
	}
	png, err := qrcode.Encode(link, qrcode.Medium, 512)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// VPSResetTraffic zeroes every user's traffic counters on the server.
func (a *App) VPSResetTraffic(id string) (o vps.Overview, err error) {
	err = a.withVPS(id, time.Minute, func(ctx context.Context, c *vps.Client) error {
		if err := c.ResetTraffic(ctx); err != nil {
			return err
		}
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	return o, err
}
