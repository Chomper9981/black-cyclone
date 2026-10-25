//go:build !linux

package tunnel

import (
	"errors"

	"golang.zx2c4.com/wireguard/tun"
)

// openTUN exists so the package builds (and its tests run) off Android.
func openTUN(int) (tun.Device, error) {
	return nil, errors.New("TUN fd chỉ hỗ trợ trên Android/Linux")
}
