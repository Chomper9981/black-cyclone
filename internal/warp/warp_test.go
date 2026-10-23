package warp

import (
	"os"
	"testing"
)

// Hits the real Cloudflare API, so it only runs with RCV_LIVE=1.
func TestRegisterLive(t *testing.T) {
	if os.Getenv("RCV_LIVE") != "1" {
		t.Skip("set RCV_LIVE=1 to call the Cloudflare API")
	}
	w, err := Register()
	if err != nil {
		t.Fatal(err)
	}
	if w.PeerPublicKey == "" || len(w.Addresses) != 2 || w.Addresses[0] == "/32" {
		t.Fatalf("incomplete registration: %+v", w)
	}
	t.Logf("peer=%s addrs=%v reserved=%v", w.PeerPublicKey, w.Addresses, w.Reserved)
}
