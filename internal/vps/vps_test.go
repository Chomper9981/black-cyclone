package vps

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestParseOverview(t *testing.T) {
	out := `@@ACTIVE active
@@SINCE Mon 2026-11-09 10:00:00 UTC
@@CONF PORT=443
@@CONF SNI=www.microsoft.com
@@CONF HOST=203.0.113.10
@@VERSION 1.14.3
@@SCRIPT abc
@@LINKS
[default]
vless://u1@203.0.113.10:443?security=reality#203.0.113.10-default

[phone]
vless://u2@203.0.113.10:443?security=reality#203.0.113.10-phone

`
	o := parseOverview(out, "abc")
	if !o.Installed || o.Active != "active" || o.Port != 443 || o.SNI != "www.microsoft.com" ||
		o.Host != "203.0.113.10" || o.Version != "1.14.3" || o.ScriptOutdated {
		t.Fatalf("bad overview: %+v", o)
	}
	if len(o.Users) != 2 || o.Users[1].Name != "phone" || !strings.HasPrefix(o.Users[1].Link, "vless://u2@") {
		t.Fatalf("bad users: %+v", o.Users)
	}
	if parseOverview(out, "other").ScriptOutdated != true {
		t.Fatal("script hash mismatch not detected")
	}
	if o := parseOverview("@@NOTINSTALLED\n", "x"); o.Installed {
		t.Fatal("not-installed parsed as installed")
	}
}

func TestInstallArgs(t *testing.T) {
	a, err := InstallOptions{Port: 8443, SNI: "www.apple.com", Host: "1.2.3.4", Name: "me"}.args()
	if err != nil || strings.Join(a, " ") != "--port 8443 --sni www.apple.com --host 1.2.3.4 --name me" {
		t.Fatalf("args = %v, %v", a, err)
	}
	for _, bad := range []InstallOptions{
		{Port: 70000}, {SNI: "x; rm -rf /"}, {SNI: "localhost"}, {Host: "$(id)"}, {Name: "a b"},
	} {
		if _, err := bad.args(); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if ValidUserName("x'y") || ValidUserName("") || !ValidUserName("laptop-me.2") {
		t.Fatal("ValidUserName")
	}
}

func TestQuote(t *testing.T) {
	if got := quote(`a'b`); got != `'a'\''b'` {
		t.Fatal(got)
	}
}

// TestLive drives a throwaway "empty VPS" (server/test/Dockerfile) end to end:
// both login kinds (root + password, sudo user + key), host key pinning,
// install, users, restart, logs and uninstall.
//
//	BC_VPS_LIVE=1 go test ./internal/vps -run Live -v -timeout 20m
func TestLive(t *testing.T) {
	if os.Getenv("BC_VPS_LIVE") == "" {
		t.Skip("set BC_VPS_LIVE=1 (needs Docker)")
	}
	const (
		name    = "bc-vps-gotest"
		image   = "blackcyclone-vps-test"
		sshPort = "2223"
		vpnPort = "18444"
		rootPW  = "Root-pass-123"
	)
	repo, _ := filepath.Abs("../..")
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	vpsSh := func(cmd string) string { return run("exec", name, "bash", "-c", cmd) }

	run("build", "-q", "-t", image, filepath.Join(repo, "server", "test"))
	_ = exec.Command("docker", "rm", "-f", name).Run()
	run("run", "-d", "--name", name, "--hostname", "fake-vps", "--privileged", "--cgroupns=host",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock",
		"-p", "127.0.0.1:"+sshPort+":22", "-p", "127.0.0.1:"+vpnPort+":"+vpnPort, image)
	if os.Getenv("BC_VPS_KEEP") == "" {
		t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	}
	for i := 0; ; i++ {
		s := strings.TrimSpace(vpsSh("systemctl is-system-running 2>/dev/null; true"))
		if s == "running" || s == "degraded" {
			break
		}
		if i > 60 {
			t.Fatal("systemd did not start: " + s)
		}
		time.Sleep(time.Second)
	}

	// Sudo user with a key, root with a password.
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, _ := ssh.MarshalPrivateKey(priv, "")
	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	sp, _ := ssh.NewPublicKey(pub)
	vpsSh("echo '" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sp))) + "' > /home/ubuntu/.ssh/authorized_keys" +
		" && chown ubuntu:ubuntu /home/ubuntu/.ssh/authorized_keys && chmod 600 /home/ubuntu/.ssh/authorized_keys" +
		" && echo 'root:" + rootPW + "' | chpasswd" +
		" && printf 'PermitRootLogin yes\\nPasswordAuthentication yes\\n' > /etc/ssh/sshd_config.d/00-test.conf" +
		" && systemctl restart ssh")

	script, sum := Script(mustRead(t, filepath.Join(repo, "server", "bcserver.sh")))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	sudoHost := Host{Addr: "127.0.0.1", Port: 2223, User: "ubuntu", KeyFile: keyFile}
	rootHost := Host{Addr: "127.0.0.1", Port: 2223, User: "root", Password: rootPW}

	// Wrong password.
	bad := rootHost
	bad.Password = "nope"
	if _, err := Dial(ctx, &bad); err == nil || !strings.Contains(err.Error(), "sai tên đăng nhập") {
		t.Fatalf("wrong password: %v", err)
	}

	c := dial(t, ctx, &sudoHost)
	if sudoHost.HostKey == "" {
		t.Fatal("host key not pinned")
	}
	if err := c.CheckRoot(ctx); err != nil {
		t.Fatal("sudo user is not root:", err)
	}
	if o, err := c.Overview(ctx, sum); err != nil || o.Installed {
		t.Fatalf("fresh VPS: %+v %v", o, err)
	}

	var lines int
	err := c.Install(ctx, script, InstallOptions{Port: 18444, Host: "127.0.0.1", Name: "tester"}, func(l string) {
		lines++
		t.Log("  | " + l)
	})
	if err != nil {
		t.Fatal("install:", err)
	}
	if lines == 0 {
		t.Error("no install output streamed")
	}
	o, err := c.Overview(ctx, sum)
	if err != nil || !o.Installed || o.Active != "active" || o.Port != 18444 || o.ScriptOutdated ||
		len(o.Users) != 1 || o.Users[0].Name != "tester" {
		t.Fatalf("after install: %+v %v", o, err)
	}
	c.Close()

	// Root with a password, through the same pinned key.
	rootHost.HostKey = sudoHost.HostKey
	c = dial(t, ctx, &rootHost)
	link, err := c.AddUser(ctx, "phone")
	if err != nil || !strings.Contains(link, "#127.0.0.1-phone") {
		t.Fatalf("add user: %q %v", link, err)
	}
	if _, err := c.AddUser(ctx, "phone"); err == nil || !strings.Contains(err.Error(), "đã có") {
		t.Fatalf("duplicate user: %v", err)
	}
	if o, _ = c.Overview(ctx, sum); len(o.Users) != 2 {
		t.Fatalf("users after add: %+v", o.Users)
	}
	if err := c.RemoveUser(ctx, "phone"); err != nil {
		t.Fatal("remove:", err)
	}
	if err := c.RemoveUser(ctx, "tester"); err == nil {
		t.Fatal("removed the last user")
	}
	if err := c.Restart(ctx); err != nil {
		t.Fatal("restart:", err)
	}
	if logs, err := c.Logs(ctx); err != nil || strings.TrimSpace(logs) == "" {
		t.Fatalf("logs: %q %v", logs, err)
	}

	// Outdated script detection: the VPS copy differs from what the app ships.
	vpsSh("echo '# old' >> /usr/local/bin/bcserver")
	if o, _ = c.Overview(ctx, sum); !o.ScriptOutdated {
		t.Error("outdated script not detected")
	}
	if err := c.Install(ctx, script, InstallOptions{}, nil); err != nil {
		t.Fatal("update:", err)
	}
	if o, _ = c.Overview(ctx, sum); o.ScriptOutdated || len(o.Users) != 1 || o.Port != 18444 {
		t.Fatalf("after update: %+v", o)
	}

	if err := c.Uninstall(ctx); err != nil {
		t.Fatal("uninstall:", err)
	}
	if o, err = c.Overview(ctx, sum); err != nil || o.Installed {
		t.Fatalf("after uninstall: %+v %v", o, err)
	}
	c.Close()

	// A different host key must be refused.
	vpsSh("rm -f /etc/ssh/ssh_host_* && ssh-keygen -A >/dev/null && systemctl restart ssh")
	var hk *HostKeyChangedError
	if _, err := Dial(ctx, &rootHost); !errors.As(err, &hk) {
		t.Fatalf("changed host key accepted: %v", err)
	}
}

func dial(t *testing.T, ctx context.Context, h *Host) *Client {
	t.Helper()
	var c *Client
	var err error
	for i := 0; i < 10; i++ { // sshd may still be restarting
		if c, err = Dial(ctx, h); err == nil {
			return c
		}
		time.Sleep(time.Second)
	}
	t.Fatal("dial:", err)
	return nil
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
