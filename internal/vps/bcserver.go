package vps

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const bin = "/usr/local/bin/bcserver"

// Same rules as bcserver.sh: values are passed on its command line.
var (
	reName   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
	reDomain = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	reHost   = regexp.MustCompile(`^[A-Za-z0-9.:-]{1,253}$`)
)

func ValidUserName(n string) bool { return reName.MatchString(n) }

// Script returns bcserver.sh with LF line endings (git may check it out as CRLF)
// and its SHA-256, used to tell whether a VPS runs an older copy.
func Script(raw []byte) (data []byte, sum string) {
	data = []byte(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	h := sha256.Sum256(data)
	return data, hex.EncodeToString(h[:])
}

// User is one user of the server and their share link.
type User struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// Overview is the state of the server on a VPS.
type Overview struct {
	Installed      bool   `json:"installed"`
	Active         string `json:"active"` // systemctl is-active: active, inactive, failed...
	Since          string `json:"since"`  // when the service last started
	Host           string `json:"host"`   // address written into links
	Port           int    `json:"port"`
	SNI            string `json:"sni"`
	Version        string `json:"version"`         // sing-box on the server
	ScriptOutdated bool   `json:"script_outdated"` // the VPS runs an older bcserver than this app ships
	Users          []User `json:"users"`
}

// Overview reads the server state in one round trip. scriptSum is the hash of
// the bcserver.sh shipped with the app.
func (c *Client) Overview(ctx context.Context, scriptSum string) (Overview, error) {
	const script = `
if [ ! -x ` + bin + ` ] || [ ! -f /etc/blackcyclone/server.conf ]; then echo "@@NOTINSTALLED"; exit 0; fi
echo "@@ACTIVE $(systemctl is-active blackcyclone 2>/dev/null)"
echo "@@SINCE $(systemctl show blackcyclone -p ActiveEnterTimestamp --value 2>/dev/null)"
grep -E '^(PORT|SNI|HOST)=' /etc/blackcyclone/server.conf | sed 's/^/@@CONF /'
echo "@@VERSION $(/opt/blackcyclone/sing-box version 2>/dev/null | head -n1 | awk '{print $3}')"
echo "@@SCRIPT $(sha256sum ` + bin + ` | cut -c1-64)"
echo "@@LINKS"
` + bin + ` link
`
	out, err := c.Run(ctx, script, nil)
	if err != nil {
		return Overview{}, err
	}
	return parseOverview(out, scriptSum), nil
}

func parseOverview(out, scriptSum string) Overview {
	var o Overview
	o.Installed = true
	links := false
	var name string
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "@@NOTINSTALLED":
			return Overview{}
		case strings.HasPrefix(line, "@@ACTIVE "):
			o.Active = strings.TrimSpace(strings.TrimPrefix(line, "@@ACTIVE "))
		case strings.HasPrefix(line, "@@SINCE "):
			o.Since = strings.TrimSpace(strings.TrimPrefix(line, "@@SINCE "))
		case strings.HasPrefix(line, "@@CONF "):
			k, v, _ := strings.Cut(strings.TrimPrefix(line, "@@CONF "), "=")
			switch k {
			case "PORT":
				o.Port, _ = strconv.Atoi(v)
			case "SNI":
				o.SNI = v
			case "HOST":
				o.Host = v
			}
		case strings.HasPrefix(line, "@@VERSION "):
			o.Version = strings.TrimSpace(strings.TrimPrefix(line, "@@VERSION "))
		case strings.HasPrefix(line, "@@SCRIPT "):
			sum := strings.TrimSpace(strings.TrimPrefix(line, "@@SCRIPT "))
			o.ScriptOutdated = scriptSum != "" && sum != scriptSum
		case line == "@@LINKS":
			links = true
		case links && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			name = line[1 : len(line)-1]
		case links && strings.HasPrefix(line, "vless://") && name != "":
			o.Users = append(o.Users, User{Name: name, Link: line})
			name = ""
		}
	}
	if o.Active == "" {
		o.Active = "unknown"
	}
	return o
}

// InstallOptions are passed to "bcserver install"; empty values keep the
// server's current (or default) setting.
type InstallOptions struct {
	Port int    `json:"port"`
	SNI  string `json:"sni"`
	Host string `json:"host"` // address written into links
	Name string `json:"name"` // first user, on a fresh install
}

func (o InstallOptions) args() ([]string, error) {
	var a []string
	if o.Port != 0 {
		if o.Port < 1 || o.Port > 65535 {
			return nil, errors.New("cổng phải từ 1 đến 65535")
		}
		a = append(a, "--port", strconv.Itoa(o.Port))
	}
	if o.SNI != "" {
		if !reDomain.MatchString(o.SNI) || !strings.Contains(o.SNI, ".") {
			return nil, errors.New("SNI phải là tên miền, ví dụ www.microsoft.com")
		}
		a = append(a, "--sni", o.SNI)
	}
	if o.Host != "" {
		if !reHost.MatchString(o.Host) {
			return nil, errors.New("địa chỉ phải là IP hoặc tên miền")
		}
		a = append(a, "--host", o.Host)
	}
	if o.Name != "" {
		if !ValidUserName(o.Name) {
			return nil, errors.New("tên người dùng chỉ gồm chữ, số, '.', '_', '-' (tối đa 32 ký tự)")
		}
		a = append(a, "--name", o.Name)
	}
	return a, nil
}

// Install uploads script (bcserver.sh) and runs "install": first setup, or an
// update that keeps keys and users. Progress lines go to onLine.
func (c *Client) Install(ctx context.Context, script []byte, opts InstallOptions, onLine func(string)) error {
	args, err := opts.args()
	if err != nil {
		return err
	}
	// The script travels inside the command (base64) so stdin stays free for sudo.
	cmd := fmt.Sprintf(`set -e
f=$(mktemp /tmp/bcserver.XXXXXX)
trap 'rm -f "$f"' EXIT
echo %s | base64 -d > "$f"
bash "$f" install %s
`, base64.StdEncoding.EncodeToString(script), strings.Join(args, " "))
	_, err = c.Run(ctx, cmd, onLine)
	return err
}

// AddUser creates a user and returns their link.
func (c *Client) AddUser(ctx context.Context, name string) (string, error) {
	if !ValidUserName(name) {
		return "", errors.New("tên chỉ gồm chữ, số, '.', '_', '-' (tối đa 32 ký tự)")
	}
	out, err := c.Run(ctx, bin+" add "+name, nil)
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "vless://") {
			return l, nil
		}
	}
	return "", errors.New("VPS không trả về link")
}

func (c *Client) RemoveUser(ctx context.Context, name string) error {
	if !ValidUserName(name) {
		return errors.New("tên không hợp lệ")
	}
	_, err := c.Run(ctx, bin+" remove "+name, nil)
	return err
}

func (c *Client) Restart(ctx context.Context) error {
	_, err := c.Run(ctx, bin+" restart", nil)
	return err
}

// Logs returns the last lines of the server log.
func (c *Client) Logs(ctx context.Context) (string, error) {
	return c.Run(ctx, "journalctl -u blackcyclone -n 200 --no-pager 2>&1", nil)
}

func (c *Client) Uninstall(ctx context.Context) error {
	_, err := c.Run(ctx, bin+" uninstall --yes", nil)
	return err
}

// CheckRoot verifies the login works and that commands really run as root.
func (c *Client) CheckRoot(ctx context.Context) error {
	out, err := c.Run(ctx, "id -u", nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "0" {
		return errors.New("không chạy được lệnh với quyền root")
	}
	return nil
}
