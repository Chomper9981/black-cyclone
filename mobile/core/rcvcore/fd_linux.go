package rcvcore

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// tunName asks the kernel for the name of the TUN interface behind fd.
func tunName(fd int) (string, error) {
	var ifr [unix.IFNAMSIZ + 64]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TUNGETIFF), uintptr(unsafe.Pointer(&ifr[0])))
	if errno != 0 {
		return "", fmt.Errorf("TUNGETIFF: %w", errno)
	}
	return unix.ByteSliceToString(ifr[:]), nil
}

func dupFd(fd int) (int, error) { return unix.Dup(fd) }

func closeFd(fd int) { _ = syscall.Close(fd) }
