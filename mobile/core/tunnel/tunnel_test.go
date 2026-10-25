package tunnel

import (
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/conn"

	"blackcyclone-vpn-mobile/core/warp"
)

type fakeBind struct {
	conn.Bind
	sent [][]byte
	in   []byte
}

func (f *fakeBind) Open(uint16) ([]conn.ReceiveFunc, uint16, error) {
	fn := func(p [][]byte, sizes []int, _ []conn.Endpoint) (int, error) {
		sizes[0] = copy(p[0], f.in)
		return 1, nil
	}
	return []conn.ReceiveFunc{fn}, 51820, nil
}

func (f *fakeBind) Send(bufs [][]byte, _ conn.Endpoint) error {
	f.sent = append(f.sent, bufs...)
	return nil
}

func TestReservedBind(t *testing.T) {
	inner := &fakeBind{in: []byte{2, 0xAA, 0xBB, 0xCC, 9, 9}}
	b := newReservedBind(inner, [3]byte{1, 2, 3})

	if err := b.Send([][]byte{{4, 0, 0, 0, 7}, {1, 0, 0, 0}}, nil); err != nil {
		t.Fatal(err)
	}
	for _, s := range inner.sent {
		if s[1] != 1 || s[2] != 2 || s[3] != 3 {
			t.Fatalf("reserved not written: %v", s)
		}
	}

	fns, _, _ := b.Open(0)
	pkt := [][]byte{make([]byte, 16)}
	sizes := []int{0}
	if n, _ := fns[0](pkt, sizes, make([]conn.Endpoint, 1)); n != 1 {
		t.Fatal("no packet")
	}
	if got := pkt[0][:sizes[0]]; got[0] != 2 || got[1] != 0 || got[2] != 0 || got[3] != 0 || got[4] != 9 {
		t.Fatalf("reserved not cleared: %v", got)
	}

	if newReservedBind(inner, [3]byte{}) != conn.Bind(inner) {
		t.Fatal("zero reserved should not wrap")
	}
}

func TestUAPIConfig(t *testing.T) {
	a := &warp.Account{
		PrivateKey:    "4JDZE/Uyuc+BbodOX49RZ0khH68/RVoRI7UKP5hiY28=",
		PeerPublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		Endpoint:      "162.159.192.1",
		EndpointPort:  2408,
		Reserved:      []int{168, 26, 21},
	}
	cfg, res, err := uapiConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if res != [3]byte{168, 26, 21} {
		t.Fatalf("reserved %v", res)
	}
	for _, want := range []string{
		"private_key=e090d913f532b9cf816e874e5f8f516749211faf3f455a1123b50a3f9862636f\n",
		"public_key=6e65ce0be17517110c17d77288ad87e7fd5252dcc7d09b95a39d61db03df832a\n",
		"endpoint=162.159.192.1:2408\n",
		"allowed_ip=::/0\n",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("missing %q in\n%s", want, cfg)
		}
	}

	a.PrivateKey = ""
	if _, _, err := uapiConfig(a); err == nil {
		t.Fatal("expected error without key")
	}
}

func TestParseStats(t *testing.T) {
	s := parseStats("public_key=ab\nrx_bytes=100\ntx_bytes=50\nlast_handshake_time_sec=1700000000\nlast_handshake_time_nsec=0\n")
	if s.RxBytes != 100 || s.TxBytes != 50 || s.LastHandshake.Unix() != 1700000000 {
		t.Fatalf("%+v", s)
	}
	if !parseStats("rx_bytes=1\n").LastHandshake.IsZero() {
		t.Fatal("handshake should be zero")
	}
}
