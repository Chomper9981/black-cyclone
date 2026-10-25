// On Linux the app starts at logon through an XDG autostart entry
// (~/.config/autostart), honoured by GNOME, KDE, XFCE and most other desktops.
// No privileges are needed: the network capabilities live on the binary itself.
package autostart

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const entryName = "blackcyclone.desktop"

// Enable writes (or replaces) the autostart entry running exePath with args.
func Enable(exePath string, args ...string) error {
	exe, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	p, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(desktopEntry(exe, args)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Disable removes the autostart entry. Removing one that does not exist is not an error.
func Disable() error {
	p, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Enabled reports whether the autostart entry exists.
func Enabled() bool {
	p, err := entryPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func entryPath() (string, error) {
	base, err := os.UserConfigDir() // $XDG_CONFIG_HOME or ~/.config
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "autostart", entryName), nil
}

// desktopEntry builds the .desktop file. The 10s delay (GNOME/KDE keys) lets
// the panel's tray host come up before the app registers its icon.
func desktopEntry(exe string, args []string) string {
	cmd := make([]string, 0, 1+len(args))
	for _, a := range append([]string{exe}, args...) {
		cmd = append(cmd, quoteExec(a))
	}
	return `[Desktop Entry]
Type=Application
Name=Black Cyclone
Comment=Start Black Cyclone at logon
Exec=` + strings.Join(cmd, " ") + `
Icon=blackcyclone
Terminal=false
X-GNOME-Autostart-Delay=10
X-KDE-autostart-after=panel
`
}

// quoteExec quotes one Exec= argument per the Desktop Entry spec: inside double
// quotes, `"`, "`", `$` and `\` are backslash-escaped and `%` is doubled (it
// introduces field codes). Exec is a string-type key, whose own escaping then
// doubles every backslash once more.
func quoteExec(a string) string {
	if a != "" && !strings.ContainsAny(a, " \t\n\"'\\><~|&;$*?#()`%=") {
		return a
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range a {
		switch r {
		case '"', '`', '$':
			b.WriteString(`\\`)
			b.WriteRune(r)
		case '\\':
			b.WriteString(`\\\\`)
		case '%':
			b.WriteString("%%")
		case '\n':
			b.WriteByte(' ') // a key value cannot span lines
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
