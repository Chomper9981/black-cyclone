// Package tray shows the notification-area icon and menu.
//
// It runs github.com/energye/systray on its own goroutine locked to a
// dedicated OS thread (the hidden tray window and its message loop must live
// on the same thread; Wails owns the main thread). Callbacks are invoked on
// fresh goroutines, never on the tray thread, so they may block or call
// back into this package.
package tray

import (
	_ "embed"
	"runtime"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/energye/systray"
)

//go:generate go run ./genicons.go

//go:embed connected.ico
var iconConnected []byte

//go:embed disconnected.ico
var iconDisconnected []byte

// Callbacks are the actions triggered from the tray. Nil entries are ignored.
type Callbacks struct {
	Show   func() // left-click / double-click on the icon, or "Mở RedCyclone"
	Toggle func() // "Kết nối" / "Ngắt kết nối"
	Quit   func() // "Thoát"
}

const (
	labelShow       = "Mở RedCyclone"
	labelConnect    = "Kết nối"
	labelDisconnect = "Ngắt kết nối"
	labelQuit       = "Thoát"
	defaultTooltip  = "RedCyclone VPN"
)

var (
	mu        sync.Mutex
	started   bool
	ready     bool
	done      chan struct{}
	connected bool
	tooltip   = defaultTooltip
	toggle    *systray.MenuItem
	icons     = [2][]byte{iconDisconnected, iconConnected}
)

// Start shows the tray icon and returns immediately. icon, if non-nil,
// replaces the embedded "connected" icon (must be .ico bytes); the
// disconnected icon is always the embedded greyscale one. Start may be
// called only once per process (the underlying library is single-shot).
func Start(icon []byte, cb Callbacks) {
	mu.Lock()
	defer mu.Unlock()
	if started {
		return
	}
	started = true
	if len(icon) > 0 {
		icons[1] = icon
	}
	done = make(chan struct{})
	call := func(f func()) func() {
		return func() {
			if f != nil {
				go f()
			}
		}
	}
	onReady := func() {
		systray.SetTitle(defaultTooltip)
		systray.SetOnClick(func(systray.IMenu) { call(cb.Show)() })
		systray.SetOnDClick(func(systray.IMenu) { call(cb.Show)() })
		// Right-click: default behaviour shows the menu.

		systray.AddMenuItem(labelShow, "").Click(call(cb.Show))
		t := systray.AddMenuItem(labelConnect, "")
		t.Click(call(cb.Toggle))
		systray.AddSeparator()
		systray.AddMenuItem(labelQuit, "").Click(call(cb.Quit))

		mu.Lock()
		toggle = t
		ready = true
		applyLocked()
		mu.Unlock()
	}
	go func() {
		runtime.LockOSThread() // window creation + GetMessage loop on one thread
		defer close(done)
		systray.Run(onReady, func() {
			mu.Lock()
			ready = false
			mu.Unlock()
		})
	}()
}

// SetState updates the icon variant, the toggle item label and the tooltip.
// An empty tooltip means "RedCyclone VPN". Safe to call any time, from any
// goroutine, including before the tray is ready (applied once it is).
func SetState(isConnected bool, tip string) {
	mu.Lock()
	defer mu.Unlock()
	connected = isConnected
	if tip == "" {
		tip = defaultTooltip
	}
	tooltip = tip
	applyLocked()
}

func applyLocked() {
	if !ready {
		return
	}
	i := 0
	label := labelConnect
	if connected {
		i, label = 1, labelDisconnect
	}
	systray.SetIcon(icons[i])
	systray.SetTooltip(truncateUTF16(tooltip, 127)) // NOTIFYICONDATA.szTip is 128 WCHARs
	if toggle != nil {
		toggle.SetTitle(label)
	}
}

// Stop removes the icon and ends the tray loop (waits up to 2s).
func Stop() {
	mu.Lock()
	if !started || done == nil {
		mu.Unlock()
		return
	}
	d := done
	mu.Unlock()
	systray.Quit()
	select {
	case <-d:
	case <-time.After(2 * time.Second):
	}
}

func truncateUTF16(s string, max int) string {
	if len(utf16.Encode([]rune(s))) <= max {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && len(utf16.Encode(r)) > max-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
