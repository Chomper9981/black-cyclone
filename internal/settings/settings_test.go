package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // Linux keeps its key in the config dir
	in := []byte(`{"private_key":"secret"}`)
	enc, err := protect(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, []byte("secret")) {
		t.Fatal("ciphertext contains plaintext")
	}
	out, err := unprotect(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("round trip mismatch: %q", out)
	}
}

func TestMigrateCustomLink(t *testing.T) {
	s := Settings{CustomLink: "vless://x@h:1", SelectedServer: AutoServer}
	s.migrate()
	if len(s.Servers) != 1 || s.CustomLink != "" || s.SelectedServer != s.Servers[0].ID {
		t.Fatalf("bad migration: %+v", s)
	}
}

func TestMigrateBypassToWarp(t *testing.T) {
	for _, m := range []Mode{"bypass", ""} {
		s := Settings{Mode: m}
		s.migrate()
		if s.Mode != ModeWarp {
			t.Errorf("%q: mode %q, want warp", m, s.Mode)
		}
	}
	s := Settings{Mode: ModeCustom}
	s.migrate()
	if s.Mode != ModeCustom {
		t.Errorf("custom mode changed to %q", s.Mode)
	}
}

func TestMigrateLegacy(t *testing.T) {
	base := t.TempDir()
	oldDir, dir := filepath.Join(base, "RedCyclone"), filepath.Join(base, "BlackCyclone")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { // e.g. created empty by an earlier run
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	migrateLegacy(oldDir, dir)
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err != nil || string(b) != "old" {
		t.Fatalf("not migrated: %q %v", b, err)
	}
	// Never overwrite settings that already exist in the new folder.
	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte("newer-old"), 0o600); err != nil {
		t.Fatal(err)
	}
	migrateLegacy(oldDir, dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "old" {
		t.Fatalf("overwrote existing settings: %q", b)
	}
}
