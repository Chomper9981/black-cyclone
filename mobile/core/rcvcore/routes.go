package rcvcore

import (
	"net/netip"
	"strings"
)

// RoutesExcluding returns, comma-separated, the CIDRs that cover 0.0.0.0/0 and ::/0
// except the comma-separated CIDRs in exclude. VpnService.Builder.excludeRoute only
// exists from Android 13, so older versions add these routes instead.
func RoutesExcluding(exclude string) string {
	routes := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}
	for _, s := range strings.Split(exclude, ",") {
		ex, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		routes = subtract(routes, ex.Masked())
	}
	out := make([]string, len(routes))
	for i, p := range routes {
		out[i] = p.String()
	}
	return strings.Join(out, ",")
}

func subtract(routes []netip.Prefix, ex netip.Prefix) []netip.Prefix {
	var out []netip.Prefix
	for _, r := range routes {
		out = append(out, subtractOne(r, ex)...)
	}
	return out
}

// subtractOne returns r minus ex as a list of prefixes.
func subtractOne(r, ex netip.Prefix) []netip.Prefix {
	if r.Addr().Is4() != ex.Addr().Is4() || !r.Overlaps(ex) {
		return []netip.Prefix{r}
	}
	if ex.Bits() <= r.Bits() {
		return nil // r lies entirely inside ex
	}
	// Split r into halves and keep subtracting from the half that holds ex.
	lo, hi := halves(r)
	return append(subtractOne(lo, ex), subtractOne(hi, ex)...)
}

func halves(p netip.Prefix) (netip.Prefix, netip.Prefix) {
	bits := p.Bits() + 1
	lo := netip.PrefixFrom(p.Addr(), bits)
	b := p.Addr().AsSlice()
	byteIdx, bit := p.Bits()/8, 7-p.Bits()%8
	b[byteIdx] |= 1 << bit
	hiAddr, _ := netip.AddrFromSlice(b)
	return lo, netip.PrefixFrom(hiAddr, bits)
}
