package i18n

import "golang.org/x/sys/windows"

// systemLocale returns the user's preferred Windows display language, e.g. "vi-VN".
func systemLocale() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
