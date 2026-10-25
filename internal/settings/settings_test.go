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

func TestDPICommand(t *testing.T) {
	s := Default()
	if got := s.DPICommand(); len(got) != 1 || got[0] != "-9" {
		t.Fatalf("default preset: %v", got)
	}
	s.DPIPreset, s.DPIArgs = "custom", " -f 2  -e 2 "
	if got := s.DPICommand(); len(got) != 4 {
		t.Fatalf("custom args: %v", got)
	}
}

func TestTLSFragment(t *testing.T) {
	for preset, want := range map[string]struct {
		mode string
		quic bool
	}{"5": {"record", false}, "9": {"tcp", false}, "9q": {"tcp", true}, "custom": {"tcp", false}} {
		s := Default()
		s.DPIPreset = preset
		if mode, quic := s.TLSFragment(); mode != want.mode || quic != want.quic {
			t.Errorf("%s: got %s/%v, want %s/%v", preset, mode, quic, want.mode, want.quic)
		}
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
