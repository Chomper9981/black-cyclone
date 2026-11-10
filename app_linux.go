package main

import (
	"os/exec"

	"black-cyclone/internal/i18n"
	"black-cyclone/internal/runner"
)

func errNotAdmin() string {
	return i18n.T(
		"app chưa có quyền tạo card mạng ảo: chạy linux/install.sh, hoặc sudo setcap cap_net_admin,cap_net_raw+ep <file BlackCyclone>",
		"the app may not create the virtual network device: run linux/install.sh, or sudo setcap cap_net_admin,cap_net_raw+ep <BlackCyclone binary>")
}

func fetchBinsHint() string {
	return i18n.T("chạy linux/fetch-bins.sh", "run linux/fetch-bins.sh")
}

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
