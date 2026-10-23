package settings

import (
	"bytes"
	"testing"
)

func TestDPAPIRoundTrip(t *testing.T) {
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
