package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, _ := windows.WaitForSingleObject(h, 0)
	return ev == uint32(windows.WAIT_TIMEOUT)
}

// TestGracefulStopCtrlBreakFallback: a child that inherited "ignore Ctrl+C"
// must still stop gracefully via the Ctrl+Break follow-up.
func TestGracefulStopCtrlBreakFallback(t *testing.T) {
	exe := findSingBox(t)
	dir := t.TempDir()
	cfg, port := writeConfig(t, dir)
	processCtrlC()
	procSetConsoleCtrlHandler.Call(0, 1) // children inherit "ignore Ctrl+C"
	p, err := Start("sing-box", exe, []string{"run", "-c", cfg, "-D", dir}, nil, nil)
	procSetConsoleCtrlHandler.Call(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	waitListening(t, port, 5*time.Second)
	begin := time.Now()
	if !p.StopGraceful() {
		t.Fatal("expected graceful stop via Ctrl+Break")
	}
	if _, err := p.Wait(0); err != nil {
		t.Fatalf("exit: %v", err)
	}
	t.Logf("stop took %v", time.Since(begin))
}

// TestGracefulStopFromConsolelessProcess runs the stop sequence inside a
// DETACHED_PROCESS helper, i.e. without a console, like the Wails GUI process.
func TestGracefulStopFromConsolelessProcess(t *testing.T) {
	exe := findSingBox(t)
	dir := t.TempDir()
	cfg, _ := writeConfig(t, dir)
	out := filepath.Join(dir, "result.txt")
	helper := exec.Command(os.Args[0], "-test.run=^TestHelperGraceful$")
	helper.Env = append(os.Environ(), "RUNNER_HELPER=2", "HELPER_EXE="+exe, "HELPER_CFG="+cfg, "HELPER_DIR="+dir, "HELPER_OUT="+out)
	helper.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
	if err := helper.Run(); err != nil {
		b, _ := os.ReadFile(out)
		t.Fatalf("helper: %v (%s)", err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "graceful=true exit=<nil>") {
		t.Fatalf("helper result: %s", b)
	}
	t.Logf("helper: %s", b)
}

func TestHelperGraceful(t *testing.T) {
	if os.Getenv("RUNNER_HELPER") != "2" {
		t.Skip("helper")
	}
	res := func(s string) { _ = os.WriteFile(os.Getenv("HELPER_OUT"), []byte(s), 0o600) }
	p, err := Start("sing-box", os.Getenv("HELPER_EXE"),
		[]string{"run", "-c", os.Getenv("HELPER_CFG"), "-D", os.Getenv("HELPER_DIR")}, nil, nil)
	if err != nil {
		res("start: " + err.Error())
		os.Exit(1)
	}
	time.Sleep(time.Second)
	begin := time.Now()
	g := p.StopGraceful()
	_, e := p.Wait(0)
	res(fmt.Sprintf("graceful=%v exit=%v took=%v", g, e, time.Since(begin)))
	os.Exit(0)
}
