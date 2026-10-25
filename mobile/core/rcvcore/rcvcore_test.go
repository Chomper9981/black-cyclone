package rcvcore

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRoutesExcluding(t *testing.T) {
	got := strings.Split(RoutesExcluding("10.0.0.0/8, 192.168.0.0/16,fe80::/10,bad"), ",")
	has := func(addr string) bool {
		a := netip.MustParseAddr(addr)
		for _, s := range got {
			if netip.MustParsePrefix(s).Contains(a) {
				return true
			}
		}
		return false
	}
	for _, in := range []string{"1.1.1.1", "9.255.255.255", "11.0.0.0", "192.167.255.255", "192.169.0.0", "2606:4700::1111"} {
		if !has(in) {
			t.Errorf("%s should be routed", in)
		}
	}
	for _, out := range []string{"10.1.2.3", "192.168.1.1", "fe80::1"} {
		if has(out) {
			t.Errorf("%s should be excluded", out)
		}
	}
	// Routes must not overlap.
	for i, a := range got {
		for _, b := range got[i+1:] {
			if netip.MustParsePrefix(a).Overlaps(netip.MustParsePrefix(b)) {
				t.Fatalf("%s overlaps %s", a, b)
			}
		}
	}
}

func TestRoutesExcludingNothing(t *testing.T) {
	if got := RoutesExcluding(""); got != "0.0.0.0/0,::/0" {
		t.Fatalf("got %q", got)
	}
}
