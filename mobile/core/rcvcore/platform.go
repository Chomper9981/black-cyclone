package rcvcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
)

// Platform is implemented by the Android VpnService and handed to SetPlatform.
type Platform interface {
	// Protect keeps a socket off the VPN (VpnService.protect).
	Protect(fd int32) bool
	// Interfaces lists the device's network interfaces as a JSON array of
	// netInterface. Go's net.Interfaces needs netlink calls that Android 11+
	// refuses to apps, so the list comes from the Java side.
	Interfaces() string
}

// netInterface is one entry of Platform.Interfaces.
type netInterface struct {
	Name      string   `json:"name"`
	Index     int      `json:"index"`
	MTU       int      `json:"mtu"`
	Type      string   `json:"type"`      // wifi, cellular, ethernet, other
	Addresses []string `json:"addresses"` // "ip/prefix"
	DNS       []string `json:"dns"`
	Metered   bool     `json:"metered"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	Multicast bool     `json:"multicast"`
}

var (
	platformMu sync.Mutex
	platformIf Platform
	// Latest default network reported by SetDefaultInterface; index -1 = none.
	defaultName    string
	defaultIndex   = -1
	defaultMetered bool
	activeMonitor  *defaultMonitor
)

// SetPlatform registers the VpnService callbacks; call it before Start.
func SetPlatform(p Platform) {
	platformMu.Lock()
	defer platformMu.Unlock()
	platformIf = p
}

// SetDefaultInterface reports the device's default network (from
// ConnectivityManager's default network callback): its interface name and
// index, or index -1 when there is none. sing-box binds its outgoing sockets to
// it and reconnects when it changes (Wi-Fi <-> mobile data).
func SetDefaultInterface(name string, index int32, metered bool) {
	platformMu.Lock()
	defaultName, defaultIndex, defaultMetered = name, int(index), metered
	m := activeMonitor
	platformMu.Unlock()
	if m != nil {
		// gomobile calls arrive on a Java thread with a small stack; do the work
		// on a goroutine like libbox does.
		done := make(chan struct{})
		go func() {
			m.update(name, int(index))
			close(done)
		}()
		<-done
	}
}

// platform implements sing-box's adapter.PlatformInterface for one session.
type platform struct {
	p              Platform
	tunFd          int // owned by the session; OpenInterface hands sing-box a dup
	networkManager adapter.NetworkManager
	myAddrs        []netip.Addr
}

var _ adapter.PlatformInterface = (*platform)(nil)

func (w *platform) Initialize(nm adapter.NetworkManager) error {
	w.networkManager = nm
	return nil
}

func (w *platform) UsePlatformAutoDetectInterfaceControl() bool { return true }

func (w *platform) AutoDetectInterfaceControl(fd int) error {
	if !w.p.Protect(int32(fd)) {
		return errors.New("VpnService.protect failed")
	}
	return nil
}

func (w *platform) UsePlatformInterface() bool { return true }

// OpenInterface wraps the TUN the VpnService already established; the routes,
// addresses and per-app rules in options were applied there.
func (w *platform) OpenInterface(options *tun.Options, _ option.TunPlatformOptions) (tun.Tun, error) {
	name, err := tunName(w.tunFd)
	if err != nil {
		return nil, fmt.Errorf("tên TUN: %w", err)
	}
	options.Name = name
	if options.InterfaceMonitor != nil {
		options.InterfaceMonitor.RegisterMyInterface(name)
	}
	fd, err := dupFd(w.tunFd)
	if err != nil {
		return nil, fmt.Errorf("dup TUN: %w", err)
	}
	options.FileDescriptor = fd
	w.myAddrs = nil
	for _, p := range append(options.Inet4Address, options.Inet6Address...) {
		w.myAddrs = append(w.myAddrs, p.Addr())
	}
	t, err := tun.New(*options)
	if err != nil {
		os.NewFile(uintptr(fd), "tun").Close()
	}
	return t, err
}

func (w *platform) ProcessPlatformOptions(option.TunPlatformOptions) error { return nil }

func (w *platform) UsePlatformDefaultInterfaceMonitor() bool { return true }

func (w *platform) CreateDefaultInterfaceMonitor(l logger.Logger) tun.DefaultInterfaceMonitor {
	return &defaultMonitor{w: w, logger: l}
}

func (w *platform) UsePlatformNetworkInterfaces() bool { return true }

func (w *platform) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	var in []netInterface
	if err := json.Unmarshal([]byte(w.p.Interfaces()), &in); err != nil {
		return nil, fmt.Errorf("danh sách interface: %w", err)
	}
	seen := map[string]bool{}
	var out []adapter.NetworkInterface
	for _, i := range in {
		if i.Name == "" || seen[i.Name] {
			continue
		}
		seen[i.Name] = true
		var flags net.Flags
		if i.Up {
			flags |= net.FlagUp | net.FlagRunning
		}
		if i.Loopback {
			flags |= net.FlagLoopback
		}
		if i.Multicast {
			flags |= net.FlagMulticast
		}
		var addrs []netip.Prefix
		for _, a := range i.Addresses {
			if p, err := netip.ParsePrefix(a); err == nil {
				addrs = append(addrs, p)
			}
		}
		typ, ok := C.StringToInterfaceType[i.Type]
		if !ok {
			typ = C.InterfaceTypeOther
		}
		out = append(out, adapter.NetworkInterface{
			Interface:  control.Interface{Index: i.Index, MTU: i.MTU, Name: i.Name, Flags: flags, Addresses: addrs},
			Type:       typ,
			DNSServers: i.DNS,
			Expensive:  i.Metered,
		})
	}
	return out, nil
}

func (w *platform) UnderNetworkExtension() bool              { return false }
func (w *platform) NetworkExtensionIncludeAllNetworks() bool { return false }
func (w *platform) ClearDNSCache()                           {}
func (w *platform) RequestPermissionForWIFIState() error     { return nil }
func (w *platform) ReadWIFIState(context.Context) adapter.WIFIState {
	return adapter.WIFIState{}
}
func (w *platform) UsePlatformConnectionOwnerFinder() bool { return false }
func (w *platform) FindConnectionOwner(*adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	return nil, os.ErrInvalid
}
func (w *platform) UsePlatformWIFIMonitor() bool                       { return false }
func (w *platform) UsePlatformNotification() bool                      { return false }
func (w *platform) SendNotification(*adapter.Notification) error       { return nil }
func (w *platform) CancelNotification(string, int32) error             { return nil }
func (w *platform) MyInterfaceAddress() []netip.Addr                   { return w.myAddrs }
func (w *platform) UsePlatformNeighborResolver() bool                  { return false }
func (w *platform) StartNeighborMonitor(adapter.NeighborUpdateListener) error { return os.ErrInvalid }
func (w *platform) CloseNeighborMonitor(adapter.NeighborUpdateListener) error { return nil }
func (w *platform) UsePlatformShell() bool                             { return false }
func (w *platform) CheckPlatformShell() error                          { return os.ErrInvalid }
func (w *platform) OpenShellSession(*adapter.PlatformUser, string, []string, string, int32, int32) (adapter.ShellSession, error) {
	return nil, os.ErrInvalid
}
func (w *platform) LookupUser(string) (*adapter.PlatformUser, error) { return nil, os.ErrInvalid }
func (w *platform) LookupSFTPServer() (string, error)                 { return "", os.ErrInvalid }
func (w *platform) ReadSystemSSHHostKey() ([]byte, error)             { return nil, os.ErrInvalid }
func (w *platform) TailscaleHostname() string                         { return "" }
func (w *platform) UsePlatformBridge() bool                           { return false }
func (w *platform) CreateBridge(adapter.BridgeOptions) (adapter.BridgeSession, error) {
	return nil, os.ErrInvalid
}

// defaultMonitor feeds sing-box the default network reported by
// SetDefaultInterface.
type defaultMonitor struct {
	w      *platform
	logger logger.Logger

	mu        sync.Mutex
	callbacks list.List[tun.DefaultInterfaceUpdateCallback]
	current   *control.Interface
	mine      []string
}

var _ tun.DefaultInterfaceMonitor = (*defaultMonitor)(nil)

func (m *defaultMonitor) Start() error {
	platformMu.Lock()
	activeMonitor = m
	name, index := defaultName, defaultIndex
	platformMu.Unlock()
	m.update(name, index)
	return nil
}

func (m *defaultMonitor) Close() error {
	platformMu.Lock()
	if activeMonitor == m {
		activeMonitor = nil
	}
	platformMu.Unlock()
	return nil
}

func (m *defaultMonitor) update(name string, index int) {
	if err := m.w.networkManager.UpdateInterfaces(); err != nil {
		m.logger.Error("update interfaces: ", err)
	}
	var iface *control.Interface
	if index >= 0 {
		var err error
		iface, err = m.w.networkManager.InterfaceFinder().ByIndex(index)
		if err != nil {
			m.logger.Error("find default interface ", name, ": ", err)
			return
		}
	}
	m.mu.Lock()
	old := m.current
	if old != nil && iface != nil && old.Equals(*iface) {
		m.mu.Unlock()
		return
	}
	m.current = iface
	callbacks := m.callbacks.Array()
	m.mu.Unlock()
	if iface == nil {
		logf("Mất kết nối mạng")
	} else if old != nil {
		logf("Mạng đổi sang %s", iface.Name)
	}
	for _, cb := range callbacks {
		cb(iface, 0)
	}
}

func (m *defaultMonitor) DefaultInterface() *control.Interface {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *defaultMonitor) OverrideAndroidVPN() bool { return false }
func (m *defaultMonitor) AndroidVPNEnabled() bool  { return false }

func (m *defaultMonitor) RegisterCallback(cb tun.DefaultInterfaceUpdateCallback) *list.Element[tun.DefaultInterfaceUpdateCallback] {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callbacks.PushBack(cb)
}

func (m *defaultMonitor) UnregisterCallback(e *list.Element[tun.DefaultInterfaceUpdateCallback]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks.Remove(e)
}

func (m *defaultMonitor) RegisterMyInterface(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mine = append(m.mine, name)
}

func (m *defaultMonitor) MyInterfaces() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mine
}
