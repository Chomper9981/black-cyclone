//go:build !windows

package i18n

import "os"

// systemLocale reads the POSIX locale variables, e.g. "vi_VN.UTF-8".
// (On Android the app sets the language explicitly instead.)
func systemLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
