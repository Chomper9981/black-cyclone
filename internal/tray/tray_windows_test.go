package tray

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

func TestEmbeddedIcons(t *testing.T) {
	for name, b := range map[string][]byte{"connected": iconConnected, "disconnected": iconDisconnected} {
		if len(b) < 6 || binary.LittleEndian.Uint16(b[2:]) != 1 {
			t.Fatalf("%s: not an .ico", name)
		}
		n := int(binary.LittleEndian.Uint16(b[4:]))
		if n < 3 {
			t.Fatalf("%s: only %d images", name, n)
		}
		for i := 0; i < n; i++ {
			e := b[6+16*i:]
			size, off := binary.LittleEndian.Uint32(e[8:]), binary.LittleEndian.Uint32(e[12:])
			img, err := png.Decode(bytes.NewReader(b[off : off+size]))
			if err != nil {
				t.Fatalf("%s #%d: %v", name, i, err)
			}
			if w := img.Bounds().Dx(); w != int(e[0]) {
				t.Fatalf("%s #%d: dir says %d, png is %d", name, i, e[0], w)
			}
		}
	}
}
