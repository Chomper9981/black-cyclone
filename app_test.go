package main

import (
	"testing"

	"redcyclone-vpn/internal/settings"
)

func TestNetworkKeyIgnoresLiveSettings(t *testing.T) {
	a := settings.Default()
	a.Mode = settings.ModeWarp
	b := a
	b.KillSwitch, b.AutoReconnect, b.Notifications = !a.KillSwitch, !a.AutoReconnect, !a.Notifications
	b.AutoConnect, b.StartWithWindows, b.StartMinimized, b.CloseToTray = true, true, true, false
	b.SelectedServer = "x"
	b.InterruptOnSwitch = !a.InterruptOnSwitch // only matters in custom mode
	if networkKey(a) != networkKey(b) {
		t.Fatal("live-only settings changed the network key")
	}
	b.DoHServer = "https://8.8.8.8/dns-query"
	if networkKey(a) == networkKey(b) {
		t.Fatal("DoH change not detected")
	}

	c := a
	c.Mode = settings.ModeCustom
	d := c
	d.InterruptOnSwitch = !c.InterruptOnSwitch
	if networkKey(c) == networkKey(d) {
		t.Fatal("interrupt_on_switch must need a reconnect in custom mode")
	}
}

// newTestApp returns an App in the given recovery state with no real processes
// (blocking is simulated; stopLocked is nil-safe).
func newTestApp(state string, blocking bool, conf settings.Settings) *App {
	a := NewApp(false)
	a.state, a.blocking, a.conf = state, blocking, conf
	return a
}

func TestApplyLiveKillSwitchOffWhileBlocked(t *testing.T) {
	conf := settings.Default()
	conf.Mode = settings.ModeWarp
	conf.AutoReconnect = false
	a := newTestApp(StateBlocked, true, conf)

	s := conf
	s.KillSwitch = false
	a.applyLiveLocked(s)
	if a.state != StateOff || a.blocking {
		t.Fatalf("state=%s blocking=%v, want off and unblocked", a.state, a.blocking)
	}
}

func TestApplyLiveAutoReconnectOffStopsRetries(t *testing.T) {
	conf := settings.Default()
	conf.Mode = settings.ModeWarp
	a := newTestApp(StateReconnecting, true, conf)
	session := a.session

	s := conf
	s.AutoReconnect = false
	a.applyLiveLocked(s)
	if a.session == session {
		t.Fatal("retry loop not cancelled")
	}
	if a.state != StateBlocked {
		t.Fatalf("state=%s, want blocked (kill switch still on)", a.state)
	}

	// Same, but without kill switch: nothing left running.
	conf.KillSwitch = false
	a = newTestApp(StateReconnecting, false, conf)
	s = conf
	s.AutoReconnect = false
	a.applyLiveLocked(s)
	if a.state != StateOff {
		t.Fatalf("state=%s, want off", a.state)
	}
}

func TestApplyLiveAutoReconnectOnWhileBlocked(t *testing.T) {
	conf := settings.Default()
	conf.Mode = settings.ModeWarp
	conf.AutoReconnect = false
	a := newTestApp(StateBlocked, true, conf)

	s := conf
	s.AutoReconnect = true
	a.applyLiveLocked(s)
	a.session++ // cancel the spawned loop before it tries to start sing-box
	if a.state != StateReconnecting {
		t.Fatalf("state=%s, want reconnecting", a.state)
	}
}
