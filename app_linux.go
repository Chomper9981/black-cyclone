package main

import (
	"os/exec"

	"redcyclone-vpn/internal/runner"
)

const (
	errNotAdmin   = "app chưa có quyền tạo card mạng ảo: chạy linux/install.sh, hoặc sudo setcap cap_net_admin,cap_net_raw+ep <file BlackCyclone>"
	fetchBinsHint = "chạy linux/fetch-bins.sh"
	hasGoodbyeDPI = false // bypass mode uses sing-box's TLS fragmentation instead
)

// isAdmin reports whether sing-box will be able to create the TUN device.
func isAdmin() bool { return runner.HasNetCaps() }

func openFolder(dir string) error {
	cmd := exec.Command("xdg-open", dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it
	return nil
}
