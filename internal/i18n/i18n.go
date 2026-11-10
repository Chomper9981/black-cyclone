// Package i18n picks between the Vietnamese and English text of a message.
//
// Translations live next to the code that uses them: i18n.T("tiếng Việt",
// "English"). There are two languages and one process-wide choice, set from
// the user's settings (or the system language when the setting is "auto").
package i18n

import (
	"strings"
	"sync/atomic"
)

const (
	Vi = "vi"
	En = "en"
)

var english atomic.Bool

// Set switches the language: "vi" or "en". Anything else resolves through the
// system language (Vietnamese systems get Vietnamese, everyone else English).
func Set(lang string) {
	english.Store(Resolve(lang) == En)
}

// Lang returns the language in use, "vi" or "en".
func Lang() string {
	if english.Load() {
		return En
	}
	return Vi
}

// Resolve maps a setting ("vi", "en", "" / "auto") to "vi" or "en".
func Resolve(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case Vi:
		return Vi
	case En:
		return En
	}
	return FromLocale(systemLocale())
}

// FromLocale maps a locale name ("vi-VN", "vi_VN.UTF-8", "en-US"...) to "vi" or "en".
func FromLocale(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	if l == Vi || strings.HasPrefix(l, "vi-") || strings.HasPrefix(l, "vi_") {
		return Vi
	}
	return En
}

// T returns vi or en according to the current language.
func T(vi, en string) string {
	if english.Load() {
		return en
	}
	return vi
}
