// Package vps manages the user's own Black Cyclone servers over SSH: it installs
// server/bcserver.sh on a VPS and drives its commands (users, links, status...).
//
// Who is an admin is decided by SSH, not by this app: whoever can log in to the
// VPS as root (or a sudo user) manages it. Users of the server only hold a
// vless:// link, which opens the proxy port and nothing else.
package vps

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"black-cyclone/internal/i18n"
)

// Host is one VPS the user manages. Stored encrypted (vps.json); the secret
// fields never leave the Go side.
type Host struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Addr string `json:"addr"` // IP or domain
	Port int    `json:"port"` // SSH port
	User string `json:"user"`
	// Password logs in when KeyFile is empty; with a key it unlocks the key if
	// the key is encrypted. Either way it is also given to sudo when User is not root.
	Password string `json:"password,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	// KeyPEM is the private key itself, used instead of KeyFile where the app
	// cannot keep a path (Android stores the key encrypted).
	KeyPEM string `json:"key_pem,omitempty"`
	// HostKey pins the server's SSH key (authorized_keys format) on first contact,
	// so a later man-in-the-middle cannot pose as the VPS.
	HostKey string `json:"host_key,omitempty"`
}

// HostKeyChangedError means the VPS answered with a different SSH key than the
// one pinned when it was added.
type HostKeyChangedError struct{ Want, Got string }

func (e *HostKeyChangedError) Error() string {
	// The UIs recognise this error by "khoá SSH của VPS đã thay đổi" / "SSH key has changed".
	return i18n.T(
		"khoá SSH của VPS đã thay đổi (trước: "+e.Want+", giờ: "+e.Got+"). "+
			"Nếu bạn vừa cài lại VPS thì chấp nhận khoá mới; nếu không, có thể có kẻ đang chặn giữa đường: đừng kết nối.",
		"the VPS's SSH key has changed (was: "+e.Want+", now: "+e.Got+"). "+
			"If you have just reinstalled the VPS, accept the new key; otherwise someone may be intercepting the connection: do not connect.")
}

// Fingerprint returns the SHA256 fingerprint of a pinned host key ("" if none or invalid).
func Fingerprint(hostKey string) string {
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(k)
}

// Client is an open SSH connection to a Host.
type Client struct {
	c    *ssh.Client
	host Host
}

// Dial connects and authenticates. If h.HostKey is empty the server key is
// pinned into h (trust on first use); otherwise it must match.
func Dial(ctx context.Context, h *Host) (*Client, error) {
	auth, err := authMethods(h)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(h.Addr, strconv.Itoa(h.Port))
	cfg := &ssh.ClientConfig{
		User: h.User,
		Auth: auth,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			got := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			if h.HostKey == "" {
				h.HostKey = got
				return nil
			}
			if h.HostKey != got {
				return &HostKeyChangedError{Want: Fingerprint(h.HostKey), Got: ssh.FingerprintSHA256(key)}
			}
			return nil
		},
		Timeout: 15 * time.Second,
	}
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("không kết nối được %s: %w", "cannot connect to %s: %w"), addr, err)
	}
	// Bound the handshake by ctx as well.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	sc, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		var hk *HostKeyChangedError
		if errors.As(err, &hk) {
			return nil, hk
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, errors.New(i18n.T("sai tên đăng nhập, mật khẩu hoặc khoá SSH", "wrong user name, password or SSH key"))
		}
		return nil, fmt.Errorf(i18n.T("SSH tới %s: %w", "SSH to %s: %w"), addr, err)
	}
	_ = conn.SetDeadline(time.Time{})
	return &Client{c: ssh.NewClient(sc, chans, reqs), host: *h}, nil
}

func authMethods(h *Host) ([]ssh.AuthMethod, error) {
	if h.KeyFile != "" || h.KeyPEM != "" {
		pem := []byte(h.KeyPEM)
		if len(pem) == 0 {
			var err error
			if pem, err = os.ReadFile(h.KeyFile); err != nil {
				return nil, fmt.Errorf(i18n.T("không đọc được file khoá: %w", "cannot read the key file: %w"), err)
			}
		}
		signer, err := ssh.ParsePrivateKey(pem)
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			if h.Password == "" {
				return nil, errors.New(i18n.T("khoá SSH có mật khẩu: nhập mật khẩu của khoá", "the SSH key is encrypted: enter its passphrase"))
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pem, []byte(h.Password))
		}
		if err != nil {
			return nil, fmt.Errorf(i18n.T("file khoá không hợp lệ: %w", "invalid key file: %w"), err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}
	if h.Password == "" {
		return nil, errors.New(i18n.T("cần mật khẩu hoặc file khoá SSH", "a password or an SSH key file is required"))
	}
	pw := h.Password
	return []ssh.AuthMethod{
		ssh.Password(pw),
		// Some servers only offer keyboard-interactive for passwords.
		ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
			ans := make([]string, len(qs))
			for i := range ans {
				ans[i] = pw
			}
			return ans, nil
		}),
	}, nil
}

func (c *Client) Close() error { return c.c.Close() }

// RunError is a remote command that exited non-zero.
type RunError struct {
	Code int
	Msg  string // last meaningful stderr line
}

func (e *RunError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf(i18n.T("lệnh trên VPS thất bại (mã %d)", "the command on the VPS failed (exit code %d)"), e.Code)
}

// Run executes script with bash as root (through sudo when the SSH user is not
// root). onLine, if set, receives stdout and stderr lines as they arrive.
// Returns stdout.
func (c *Client) Run(ctx context.Context, script string, onLine func(string)) (string, error) {
	sess, err := c.c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	// bcserver answers in the app's language.
	cmd := "bash -c " + quote("export BC_LANG="+i18n.Lang()+"; "+script)
	if c.host.User != "root" {
		if c.host.Password != "" {
			// -S reads the password from stdin; NOPASSWD sudo simply ignores it.
			cmd = "sudo -S -p '' " + cmd
			sess.Stdin = strings.NewReader(c.host.Password + "\n")
		} else {
			cmd = "sudo -n " + cmd
		}
	}

	var stdout bytes.Buffer
	var mu sync.Mutex
	var lastErr string
	outR, err := sess.StdoutPipe()
	if err != nil {
		return "", err
	}
	errR, err := sess.StderrPipe()
	if err != nil {
		return "", err
	}
	var wg sync.WaitGroup
	pump := func(r io.Reader, isErr bool) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			if isErr {
				if t := strings.TrimSpace(line); t != "" {
					lastErr = t
				}
			} else {
				stdout.WriteString(line)
				stdout.WriteByte('\n')
			}
			mu.Unlock()
			if onLine != nil {
				onLine(line)
			}
		}
	}
	wg.Add(2)
	go pump(outR, false)
	go pump(errR, true)

	if err := sess.Start(cmd); err != nil {
		return "", err
	}
	stop := context.AfterFunc(ctx, func() { sess.Close() })
	defer stop()
	wg.Wait()
	err = sess.Wait()
	if ctx.Err() != nil {
		return stdout.String(), errors.New(i18n.T("hết thời gian chờ VPS trả lời", "timed out waiting for the VPS"))
	}
	if err != nil {
		var ee *ssh.ExitError
		if errors.As(err, &ee) {
			// bcserver prefixes its errors with "Lỗi: " or "Error: ".
			msg := strings.TrimPrefix(strings.TrimPrefix(lastErr, "Lỗi: "), "Error: ")
			if strings.Contains(lastErr, "a password is required") || strings.Contains(lastErr, "incorrect password") {
				msg = i18n.T(
					"user "+c.host.User+" cần mật khẩu sudo: nhập mật khẩu khi thêm VPS (hoặc dùng root)",
					"user "+c.host.User+" needs a sudo password: enter it when adding the VPS (or use root)")
			}
			return stdout.String(), &RunError{Code: ee.ExitStatus(), Msg: msg}
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

// quote wraps s in single quotes for a POSIX shell.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
