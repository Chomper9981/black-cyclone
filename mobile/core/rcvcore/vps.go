package rcvcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"black-cyclone/internal/vps"
	"black-cyclone/server"
)

// VPS management for the Android app: the same internal/vps code as the
// desktop "VPS" tab. The app keeps each VPS as JSON (vps.Host: addr, port,
// user, password, key_pem, host_key...) encrypted with the Android Keystore
// and passes it in on every call; nothing is stored on the Go side.

var bcserverScript, bcserverSum = vps.Script(server.Script)

// VpsOutput receives install progress lines (implemented in Kotlin).
type VpsOutput interface {
	Line(line string)
}

func decodeHost(hostJSON string) (*vps.Host, error) {
	var h vps.Host
	if err := json.Unmarshal([]byte(hostJSON), &h); err != nil {
		return nil, errors.New("dữ liệu VPS không hợp lệ")
	}
	h.Addr = strings.Trim(strings.TrimSpace(h.Addr), "[]")
	if h.Addr == "" || strings.ContainsAny(h.Addr, " /@") {
		return nil, errors.New("nhập IP hoặc tên miền của VPS")
	}
	if h.Port == 0 {
		h.Port = 22
	}
	if h.Port < 1 || h.Port > 65535 {
		return nil, errors.New("cổng SSH phải từ 1 đến 65535")
	}
	if h.User == "" {
		h.User = "root"
	}
	return &h, nil
}

func encode(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

// withVps connects to the host and runs fn. A host without a pinned key is refused.
func withVps(hostJSON string, timeout time.Duration, fn func(ctx context.Context, c *vps.Client) error) error {
	h, err := decodeHost(hostJSON)
	if err != nil {
		return err
	}
	if h.HostKey == "" {
		return errors.New("VPS chưa có khoá SSH đã xác minh: xoá rồi thêm lại")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := vps.Dial(ctx, h)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(ctx, c)
}

// VpsCheck logs in and checks root rights. With trustNewKey (or no pinned key
// yet) the server's current SSH key is pinned. Returns the host JSON to save.
func VpsCheck(hostJSON string, trustNewKey bool) (string, error) {
	h, err := decodeHost(hostJSON)
	if err != nil {
		return "", err
	}
	if trustNewKey {
		h.HostKey = ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	logf("Đang kiểm tra đăng nhập SSH vào %s…", h.Addr)
	c, err := vps.Dial(ctx, h)
	if err != nil {
		logf("SSH thất bại: %v", err)
		return "", err
	}
	defer c.Close()
	if err := c.CheckRoot(ctx); err != nil {
		return "", err
	}
	logf("VPS %s: khoá SSH %s", h.Addr, vps.Fingerprint(h.HostKey))
	return encode(h)
}

// VpsFingerprint returns the SHA256 fingerprint of a pinned host key.
func VpsFingerprint(hostKey string) string { return vps.Fingerprint(hostKey) }

// VpsOverview returns the server state (vps.Overview JSON).
func VpsOverview(hostJSON string) (string, error) {
	var o vps.Overview
	err := withVps(hostJSON, 45*time.Second, func(ctx context.Context, c *vps.Client) (err error) {
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	if err != nil {
		return "", err
	}
	return encode(o)
}

// overviewAfter runs an action, then returns the new overview.
func overviewAfter(hostJSON string, timeout time.Duration, action func(ctx context.Context, c *vps.Client) error) (string, error) {
	var o vps.Overview
	err := withVps(hostJSON, timeout, func(ctx context.Context, c *vps.Client) error {
		if err := action(ctx, c); err != nil {
			return err
		}
		var err error
		o, err = c.Overview(ctx, bcserverSum)
		return err
	})
	if err != nil {
		return "", err
	}
	return encode(o)
}

// VpsInstall installs or updates the server. optsJSON is vps.InstallOptions
// (port, sni, host, name; empty values keep the current ones).
func VpsInstall(hostJSON, optsJSON string, out VpsOutput) (string, error) {
	var opts vps.InstallOptions
	if optsJSON != "" {
		if err := json.Unmarshal([]byte(optsJSON), &opts); err != nil {
			return "", errors.New("tuỳ chọn cài đặt không hợp lệ")
		}
	}
	return overviewAfter(hostJSON, 10*time.Minute, func(ctx context.Context, c *vps.Client) error {
		logf("Đang cài / cập nhật server trên VPS…")
		err := c.Install(ctx, bcserverScript, opts, func(line string) {
			if out != nil {
				out.Line(line)
			}
		})
		if err != nil {
			logf("Cài server thất bại: %v", err)
		}
		return err
	})
}

func VpsAddUser(hostJSON, name string) (string, error) {
	return overviewAfter(hostJSON, time.Minute, func(ctx context.Context, c *vps.Client) error {
		_, err := c.AddUser(ctx, strings.TrimSpace(name))
		return err
	})
}

func VpsRemoveUser(hostJSON, name string) (string, error) {
	return overviewAfter(hostJSON, time.Minute, func(ctx context.Context, c *vps.Client) error {
		return c.RemoveUser(ctx, name)
	})
}

func VpsRestart(hostJSON string) (string, error) {
	return overviewAfter(hostJSON, time.Minute, func(ctx context.Context, c *vps.Client) error {
		return c.Restart(ctx)
	})
}

func VpsResetTraffic(hostJSON string) (string, error) {
	return overviewAfter(hostJSON, time.Minute, func(ctx context.Context, c *vps.Client) error {
		return c.ResetTraffic(ctx)
	})
}

func VpsUninstall(hostJSON string) (string, error) {
	return overviewAfter(hostJSON, 2*time.Minute, func(ctx context.Context, c *vps.Client) error {
		return c.Uninstall(ctx)
	})
}

// VpsLogs returns the last lines of the server log.
func VpsLogs(hostJSON string) (string, error) {
	var out string
	err := withVps(hostJSON, time.Minute, func(ctx context.Context, c *vps.Client) (err error) {
		out, err = c.Logs(ctx)
		return err
	})
	return out, err
}

// LinkQR renders a share link as a QR code (PNG bytes).
func LinkQR(link string, size int) ([]byte, error) {
	if !strings.Contains(link, "://") || len(link) > 2048 {
		return nil, errors.New("link không hợp lệ")
	}
	if size < 128 || size > 2048 {
		size = 512
	}
	return qrcode.Encode(link, qrcode.Medium, size)
}
