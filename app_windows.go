package main

import (
	"os/exec"

	"golang.org/x/sys/windows"

	"black-cyclone/internal/i18n"
)

func errNotAdmin() string {
	return i18n.T("cần chạy ứng dụng với quyền Administrator", "the app must run as Administrator")
}

func fetchBinsHint() string {
	return i18n.T(`chạy windows\fetch-bins.ps1`, `run windows\fetch-bins.ps1`)
}

func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func openFolder(dir string) error {
	return exec.Command("explorer.exe", dir).Start()
}
