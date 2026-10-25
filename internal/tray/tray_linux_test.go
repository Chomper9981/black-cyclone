package tray

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEmbeddedIcons(t *testing.T) {
	for name, b := range map[string][]byte{"connected": iconConnected, "disconnected": iconDisconnected} {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if w := img.Bounds().Dx(); w < 32 {
			t.Fatalf("%s: %dpx, too small for HiDPI panels", name, w)
		}
	}
}
