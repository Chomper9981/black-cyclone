package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnableDisable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if Enabled() {
		t.Fatal("enabled before Enable")
	}
	exe := "/opt/Black Cyclone/BlackCyclone"
	if err := Enable(exe, "--autostart"); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("not enabled after Enable")
	}
	p, _ := entryPath()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nExec=\"/opt/Black Cyclone/BlackCyclone\" --autostart\n") {
		t.Fatalf("bad entry:\n%s", b)
	}
	if err := Enable(exe); err != nil { // replace in place
		t.Fatal(err)
	}
	if err := Disable(); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("still enabled after Disable")
	}
	if err := Disable(); err != nil {
		t.Fatalf("second Disable: %v", err)
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.tmp")); len(m) > 0 {
		t.Fatalf("temp files left: %v", m)
	}
}

func TestQuoteExec(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/app": "/usr/bin/app",
		"--autostart":  "--autostart",
		"/a b/app":     `"/a b/app"`,
		`say "hi" $X`:  `"say \\"hi\\" \\$X"`,
		`back\slash`:   `"back\\\\slash"`,
		"100%":         `"100%%"`,
		"":             `""`,
	} {
		if got := quoteExec(in); got != want {
			t.Errorf("quoteExec(%q) = %s, want %s", in, got, want)
		}
	}
}
