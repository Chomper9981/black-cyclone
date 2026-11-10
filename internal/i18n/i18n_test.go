package i18n

import "testing"

func TestLanguage(t *testing.T) {
	for locale, want := range map[string]string{
		"vi-VN": Vi, "vi_VN.UTF-8": Vi, "vi": Vi, "VI-vn": Vi,
		"en-US": En, "": En, "C": En, "fr_FR.UTF-8": En, "vie": En,
	} {
		if got := FromLocale(locale); got != want {
			t.Errorf("FromLocale(%q) = %q, want %q", locale, got, want)
		}
	}
	if Resolve("en") != En || Resolve(" VI ") != Vi {
		t.Fatal("explicit setting ignored")
	}
	defer Set(Vi)
	Set(En)
	if T("xin chào", "hello") != "hello" || Lang() != En {
		t.Fatal("English not selected")
	}
	Set(Vi)
	if T("xin chào", "hello") != "xin chào" || Lang() != Vi {
		t.Fatal("Vietnamese not selected")
	}
}
