package tunnel

import "golang.zx2c4.com/wireguard/tun"

func openTUN(fd int) (tun.Device, error) {
	dev, _, err := tun.CreateUnmonitoredTUNFromFD(fd)
	return dev, err
}
