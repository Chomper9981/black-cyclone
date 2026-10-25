// Package tunnel runs a userspace WireGuard device on an Android TUN fd for WARP.
package tunnel

import (
	"golang.zx2c4.com/wireguard/conn"
)

// reservedBind writes Cloudflare's 3-byte client_id into the reserved header bytes
// (1..3) of every outgoing WireGuard message and clears them on incoming ones, so
// wireguard-go, which reads the first 4 bytes as the message type, sees a standard
// header. MACs are computed by wireguard-go before Send, i.e. over zeroed bytes,
// which is what Cloudflare expects.
type reservedBind struct {
	conn.Bind
	reserved [3]byte
}

func newReservedBind(inner conn.Bind, reserved [3]byte) conn.Bind {
	if reserved == [3]byte{} {
		return inner
	}
	return &reservedBind{Bind: inner, reserved: reserved}
}

func (b *reservedBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	fns, actual, err := b.Bind.Open(port)
	if err != nil {
		return nil, 0, err
	}
	wrapped := make([]conn.ReceiveFunc, len(fns))
	for i, fn := range fns {
		wrapped[i] = func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
			n, err := fn(packets, sizes, eps)
			for j := 0; j < n; j++ {
				if sizes[j] >= 4 {
					packets[j][1], packets[j][2], packets[j][3] = 0, 0, 0
				}
			}
			return n, err
		}
	}
	return wrapped, actual, nil
}

func (b *reservedBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	for _, buf := range bufs {
		if len(buf) >= 4 {
			copy(buf[1:4], b.reserved[:])
		}
	}
	return b.Bind.Send(bufs, ep)
}
