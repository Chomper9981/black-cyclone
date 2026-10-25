package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const exeSuffix = ""

// Required lists the files that must exist in the bin directory. There is no
// GoodbyeDPI on Linux: bypass mode uses sing-box's own TLS fragmentation.
var Required = []string{"sing-box"}

// netCaps are what sing-box needs without root: CAP_NET_ADMIN for the TUN
// device, routes and fwmarks, CAP_NET_RAW for SO_BINDTODEVICE on older kernels.
// Granted to the app binary with `setcap cap_net_admin,cap_net_raw+ep`.
var netCaps = []uintptr{unix.CAP_NET_ADMIN, unix.CAP_NET_RAW}

// HasNetCaps reports whether this process may create the TUN device: it runs
// as root or holds CAP_NET_ADMIN.
func HasNetCaps() bool {
	return os.Geteuid() == 0 || slices.Contains(heldCaps(), unix.CAP_NET_ADMIN)
}

// heldCaps returns the members of netCaps in our permitted set.
func heldCaps() []uintptr {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return nil
	}
	var out []uintptr
	for _, c := range netCaps {
		if data[c/32].Permitted&(1<<(c%32)) != 0 {
			out = append(out, c)
		}
	}
	return out
}

// prepare ties the child to us and hands it our network capabilities.
//
// Pdeathsig sends SIGTERM (so sing-box still removes its routes) when we die,
// even by SIGKILL. The kernel keeps it across exec because the child gains no
// privileges there: its capabilities come from our permitted set via the
// ambient set, not from file capabilities on sing-box (those would clear it).
// Root needs no ambient set: the child is root as well.
//
// The signal is tied to the OS thread that forked the child, which Go only
// retires when a goroutine exits while locked to it; Start is never called
// from such a goroutine.
func prepare(cmd *exec.Cmd) {
	attr := &syscall.SysProcAttr{
		Setpgid:   true, // a terminal's Ctrl+C (`wails dev`) reaches us, not the core directly
		Pdeathsig: syscall.SIGTERM,
	}
	if os.Geteuid() != 0 {
		attr.AmbientCaps = heldCaps()
	}
	cmd.SysProcAttr = attr
}

func afterStart(string, *exec.Cmd, func(string)) error { return nil }

// interrupt sends SIGTERM; sing-box shuts down cleanly on it.
func interrupt(pid int, _ <-chan struct{}) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// ---- Stale process cleanup ------------------------------------------------

// staleNames are the helper images KillStale looks for.
var staleNames = []string{"sing-box"}

// KillStale terminates leftover sing-box processes whose image lives inside
// binDir (e.g. orphaned by a crashed instance). Children started by Start are
// left alone. Each one gets SIGTERM so it can remove its routes, then SIGKILL
// after GracefulTimeout. It returns the number of processes stopped; err
// aggregates per-process failures.
func KillStale(binDir string) (int, error) {
	want, err := filepath.Abs(binDir)
	if err != nil {
		return 0, err
	}
	want = filepath.Clean(want)
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	self := os.Getpid()
	killed := 0
	var errs []error
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self || isLive(pid) {
			continue
		}
		// Unreadable for other users' processes: those are not ours to kill.
		img, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil {
			continue
		}
		img = strings.TrimSuffix(img, " (deleted)") // binary replaced by an update
		if !slices.Contains(staleNames, filepath.Base(img)) || filepath.Clean(filepath.Dir(img)) != want {
			continue
		}
		if err := terminate(pid); err != nil {
			errs = append(errs, fmt.Errorf("kill %d (%s): %w", pid, img, err))
			continue
		}
		killed++
	}
	return killed, errors.Join(errs...)
}

func terminate(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(GracefulTimeout)
	for alive(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// alive reports whether pid exists and is not a zombie.
func alive(pid int) bool {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	// "pid (comm) state ...": comm may contain spaces and parentheses.
	i := strings.LastIndexByte(string(b), ')')
	return i >= 0 && i+2 < len(b) && b[i+2] != 'Z'
}
