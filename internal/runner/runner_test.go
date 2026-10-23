package runner

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// findSingBox locates bin\sing-box.exe by walking up from the test directory
// (works from the repo and from .claude\worktrees\<x>\ inside it).
func findSingBox(t testing.TB) string {
	if p := os.Getenv("SINGBOX_EXE"); p != "" {
		return p
	}
	dir, _ := os.Getwd()
	for {
		p := filepath.Join(dir, "bin", "sing-box.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("sing-box.exe not found (set SINGBOX_EXE)")
		}
		dir = parent
	}
}

func freePort(t testing.TB) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func writeConfig(t testing.TB, dir string) (string, int) {
	port := freePort(t)
	cfg := fmt.Sprintf(`{"log":{"level":"info"},"inbounds":[{"type":"mixed","tag":"in","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"direct","tag":"direct"}]}`, port)
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return p, port
}

func waitListening(t testing.TB, port int, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("sing-box did not start listening on %d", port)
}

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, _ := windows.WaitForSingleObject(h, 0)
	return ev == uint32(windows.WAIT_TIMEOUT)
}

func TestGracefulStop(t *testing.T) {
	exe := findSingBox(t)
	dir := t.TempDir()
	cfg, port := writeConfig(t, dir)

	var mu sync.Mutex
	var lines []string
	exitCalled := make(chan error, 1)
	p, err := Start("sing-box", exe, []string{"run", "-c", cfg, "-D", dir}, func(l string) {
		mu.Lock()
		lines = append(lines, l)
		mu.Unlock()
	}, func(err error) { exitCalled <- err })
	if err != nil {
		t.Fatal(err)
	}
	waitListening(t, port, 5*time.Second)

	begin := time.Now()
	graceful := p.StopGraceful()
	took := time.Since(begin)
	_, exitErr := p.Wait(0)
	mu.Lock()
	log := strings.Join(lines, "\n")
	mu.Unlock()
	t.Logf("stop took %v, graceful=%v, exit=%v\n%s", took, graceful, exitErr, log)

	if !graceful {
		t.Fatalf("expected graceful stop, got kill (exit=%v)", exitErr)
	}
	if exitErr != nil {
		t.Fatalf("expected exit code 0, got %v", exitErr)
	}
	if took > 3*time.Second {
		t.Fatalf("graceful stop too slow: %v", took)
	}
	if p.Running() {
		t.Fatal("still running")
	}
	select {
	case err := <-exitCalled:
		t.Fatalf("onExit must not fire on Stop, got %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	// The test process must survive (we swallowed the broadcast Ctrl+C) and a
	// second cycle must work too.
	p2, err := Start("sing-box", exe, []string{"run", "-c", cfg, "-D", dir}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitListening(t, port, 5*time.Second)
	if !p2.StopGraceful() {
		t.Fatal("second graceful stop failed")
	}
	p2.Stop() // idempotent
	var nilProc *Proc
	nilProc.Stop()
}

func TestOnExitFiresWhenChildDiesByItself(t *testing.T) {
	exe := findSingBox(t)
	got := make(chan error, 1)
	p, err := Start("sing-box", exe, []string{"run", "-c", filepath.Join(t.TempDir(), "missing.json")}, nil, func(err error) { got <- err })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if err == nil {
			t.Fatal("expected non-nil exit error")
		}
	case <-time.After(5 * time.Second):
		p.Stop()
		t.Fatal("onExit not called")
	}
	if p.PID() == 0 {
		t.Fatal("PID 0")
	}
}

// TestJobKillsChildren re-executes the test binary as a helper that starts
// sing-box through Start and then gets TerminateProcess'd; the child must die.
func TestJobKillsChildren(t *testing.T) {
	exe := findSingBox(t)
	dir := t.TempDir()
	cfg, port := writeConfig(t, dir)

	helper := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	helper.Env = append(os.Environ(), "RUNNER_HELPER=1", "HELPER_EXE="+exe, "HELPER_CFG="+cfg, "HELPER_DIR="+dir)
	out, _ := helper.StdoutPipe()
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(out)
	childPID := 0
	for sc.Scan() {
		if s, ok := strings.CutPrefix(sc.Text(), "CHILDPID="); ok {
			childPID, _ = strconv.Atoi(s)
			break
		}
	}
	go io.Copy(io.Discard, out)
	if childPID == 0 {
		helper.Process.Kill()
		t.Fatal("helper did not report child pid")
	}
	waitListening(t, port, 5*time.Second)
	if !processAlive(childPID) {
		t.Fatal("child not alive before helper kill")
	}
	_ = helper.Process.Kill() // TerminateProcess: no cleanup code runs in the helper
	_ = helper.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for processAlive(childPID) {
		if time.Now().After(deadline) {
			t.Fatalf("child %d survived its parent being killed", childPID)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("RUNNER_HELPER") != "1" {
		t.Skip("helper")
	}
	p, err := Start("sing-box", os.Getenv("HELPER_EXE"),
		[]string{"run", "-c", os.Getenv("HELPER_CFG"), "-D", os.Getenv("HELPER_DIR")}, nil, nil)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	fmt.Printf("CHILDPID=%d\n", p.PID())
	time.Sleep(time.Minute)
	os.Exit(0)
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

// TestKillStale runs a copy of sing-box from a temp dir outside our job and
// checks KillStale(tempdir) kills it (and only it).
func TestKillStale(t *testing.T) {
	src := findSingBox(t)
	dir := t.TempDir()
	dst := filepath.Join(dir, "sing-box.exe")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, port := writeConfig(t, dir)

	// One of ours (in the job) from the same dir: must survive.
	ours, err := Start("sing-box", dst, []string{"run", "-c", cfg, "-D", dir}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ours.Stop()
	waitListening(t, port, 5*time.Second)

	cfg2 := filepath.Join(dir, "config2.json")
	port2 := freePort(t)
	os.WriteFile(cfg2, []byte(fmt.Sprintf(`{"inbounds":[{"type":"mixed","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"direct"}]}`, port2)), 0o600)
	stale := exec.Command(dst, "run", "-c", cfg2, "-D", dir)
	stale.SysProcAttr = nil
	if err := stale.Start(); err != nil {
		t.Fatal(err)
	}
	waitListening(t, port2, 5*time.Second)

	n, err := KillStale(dir)
	if err != nil {
		t.Logf("KillStale errors (other processes): %v", err)
	}
	if n != 1 {
		stale.Process.Kill()
		t.Fatalf("killed %d, want 1", n)
	}
	done := make(chan struct{})
	go func() { stale.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stale process still running")
	}
	if !ours.Running() {
		t.Fatal("KillStale killed our own child")
	}
}
