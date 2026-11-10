package vps

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"black-cyclone/internal/i18n"
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
	// Traffic between the VPS and the Internet for this user, in bytes, since
	// the stats began or were reset (when Overview.Stats).
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
	// Open connections right now and the addresses they come from (when
	// Overview.Online). Many addresses at once can mean the link was shared.
	Conns int      `json:"conns"`
	IPs   []string `json:"ips"`
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
	Stats          bool   `json:"stats"`           // per-user traffic is available
	Online         bool   `json:"online"`          // open connections are available
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
echo "@@TRAFFIC"
` + bin + ` traffic 2>/dev/null || true
echo "@@ONLINE $(` + bin + ` online 2>/dev/null | tr -d '
')"
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
	links, traffic := false, false
	var name string
	type counters struct{ up, down int64 }
	stats := map[string]counters{}
	var online map[string]*User
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
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
		case line == "@@TRAFFIC":
			traffic = true
		case strings.HasPrefix(line, "@@ONLINE"):
			traffic = false
			online = parseOnline(strings.TrimSpace(strings.TrimPrefix(line, "@@ONLINE")))
			o.Online = online != nil
		case traffic:
			f := strings.Fields(line)
			if len(f) == 3 {
				up, e1 := strconv.ParseInt(f[1], 10, 64)
				down, e2 := strconv.ParseInt(f[2], 10, 64)
				if e1 == nil && e2 == nil {
					stats[f[0]] = counters{up, down}
					o.Stats = true
				}
			}
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
	for i := range o.Users {
		u := &o.Users[i]
		c := stats[u.Name]
		u.Up, u.Down = c.up, c.down
		if on := online[u.Name]; on != nil {
			u.Conns, u.IPs = on.Conns, on.IPs
		}
	}
	return o
}

// parseOnline reads the Clash API /connections JSON. Each user leaves through
// the outbound "u-<name>", so a connection's chain names its user. Returns nil
// if the JSON is missing or invalid.
func parseOnline(js string) map[string]*User {
	if js == "" {
		return nil
	}
	var r struct {
		Connections []struct {
			Chains   []string `json:"chains"`
			Metadata struct {
				SourceIP string `json:"sourceIP"`
			} `json:"metadata"`
		} `json:"connections"`
	}
	if err := json.Unmarshal([]byte(js), &r); err != nil {
		return nil
	}
	out := map[string]*User{}
	for _, c := range r.Connections {
		for _, tag := range c.Chains {
			name, ok := strings.CutPrefix(tag, "u-")
			if !ok {
				continue
			}
			u := out[name]
			if u == nil {
				u = &User{Name: name}
				out[name] = u
			}
			u.Conns++
			if ip := c.Metadata.SourceIP; ip != "" && !contains(u.IPs, ip) {
				u.IPs = append(u.IPs, ip)
			}
			break
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ResetTraffic zeroes every user's traffic counters.
func (c *Client) ResetTraffic(ctx context.Context) error {
	_, err := c.Run(ctx, bin+" traffic --reset", nil)
	return err
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
			return nil, errors.New(i18n.T("cổng phải từ 1 đến 65535", "the port must be between 1 and 65535"))
		}
		a = append(a, "--port", strconv.Itoa(o.Port))
	}
	if o.SNI != "" {
		if !reDomain.MatchString(o.SNI) || !strings.Contains(o.SNI, ".") {
			return nil, errors.New(i18n.T("SNI phải là tên miền, ví dụ www.microsoft.com", "the SNI must be a domain, e.g. www.microsoft.com"))
		}
		a = append(a, "--sni", o.SNI)
	}
	if o.Host != "" {
		if !reHost.MatchString(o.Host) {
			return nil, errors.New(i18n.T("địa chỉ phải là IP hoặc tên miền", "the address must be an IP or a domain"))
		}
		a = append(a, "--host", o.Host)
	}
	if o.Name != "" {
		if !ValidUserName(o.Name) {
			return nil, errors.New(i18n.T("tên người dùng chỉ gồm chữ, số, '.', '_', '-' (tối đa 32 ký tự)", "user names may only contain letters, digits, '.', '_', '-' (32 characters at most)"))
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
		return "", errors.New(i18n.T("tên chỉ gồm chữ, số, '.', '_', '-' (tối đa 32 ký tự)", "names may only contain letters, digits, '.', '_', '-' (32 characters at most)"))
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
	return "", errors.New(i18n.T("VPS không trả về link", "the VPS did not return a link"))
}

func (c *Client) RemoveUser(ctx context.Context, name string) error {
	if !ValidUserName(name) {
		return errors.New(i18n.T("tên không hợp lệ", "invalid name"))
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
		return errors.New(i18n.T("không chạy được lệnh với quyền root", "cannot run commands as root"))
	}
	return nil
}
