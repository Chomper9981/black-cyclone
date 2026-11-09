package rcvcore

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeHost(t *testing.T) {
	h, err := decodeHost(`{"addr":" [2001:db8::1] ","password":"x"}`)
	if err != nil || h.Addr != "2001:db8::1" || h.Port != 22 || h.User != "root" {
		t.Fatalf("defaults: %+v %v", h, err)
	}
	for _, bad := range []string{`{}`, `{"addr":"a b"}`, `{"addr":"u@h"}`, `{"addr":"h","port":70000}`, `not json`} {
		if _, err := decodeHost(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	// No pinned key: management calls refuse instead of trusting a new key silently.
	if _, err := VpsOverview(`{"addr":"192.0.2.1","password":"x"}`); err == nil || !strings.Contains(err.Error(), "khoá SSH") {
		t.Fatalf("unpinned host: %v", err)
	}
}

func TestLinkQR(t *testing.T) {
	png, err := LinkQR("vless://uuid@203.0.113.10:443?security=reality#x", 256)
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Fatalf("qr: %v", err)
	}
	if _, err := LinkQR("not a link", 256); err == nil {
		t.Fatal("accepted a non-link")
	}
}
