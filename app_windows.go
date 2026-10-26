package main

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

const (
	errNotAdmin   = "cần chạy ứng dụng với quyền Administrator"
	fetchBinsHint = `chạy windows\fetch-bins.ps1`
	hasGoodbyeDPI = true
)

func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func openFolder(dir string) error {
	return exec.Command("explorer.exe", dir).Start()
}
