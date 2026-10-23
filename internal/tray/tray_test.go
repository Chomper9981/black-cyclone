package tray

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
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

func TestTruncate(t *testing.T) {
	s := strings.Repeat("Kết nối ", 40)
	got := truncateUTF16(s, 127)
	if n := len(utf16.Encode([]rune(got))); n > 127 {
		t.Fatalf("len %d", n)
	}
	if truncateUTF16("abc", 127) != "abc" {
		t.Fatal("short string changed")
	}
}

// TestSmoke really creates the tray icon for ~2s, flips the state, then quits.
// Set TRAY_SMOKE=0 to skip (e.g. no interactive desktop).
func TestSmoke(t *testing.T) {
	if os.Getenv("TRAY_SMOKE") == "0" {
		t.Skip("TRAY_SMOKE=0")
	}
	Start(nil, Callbacks{Show: func() {}, Toggle: func() {}, Quit: func() {}})
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		r := ready
		mu.Unlock()
		if r {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tray not ready after 3s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	SetState(true, "RedCyclone VPN — đã kết nối")
	time.Sleep(time.Second)
	mu.Lock()
	title := toggle.String()
	mu.Unlock()
	if !strings.Contains(title, labelDisconnect) {
		t.Fatalf("toggle label = %s", title)
	}
	SetState(false, strings.Repeat("x", 300))
	time.Sleep(time.Second)
	Stop()
	select {
	case <-done:
	default:
		t.Fatal("tray loop did not exit")
	}
}
