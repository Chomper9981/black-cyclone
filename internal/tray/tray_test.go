package tray

import (
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

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
	SetState(true, "Black Cyclone — đã kết nối")
	time.Sleep(time.Second)
	mu.Lock()
	title := toggle.String()
	mu.Unlock()
	if !strings.Contains(title, labelDisconnect()) {
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
