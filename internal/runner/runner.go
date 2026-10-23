// Package runner starts and stops the helper processes (sing-box, GoodbyeDPI).
package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Required lists the files that must exist in the bin directory.
var Required = []string{"sing-box.exe", "goodbyedpi.exe", "WinDivert.dll", "WinDivert64.sys"}

// BinDir finds the folder holding the helper binaries: next to the exe (installed build),
// or the project's bin\ when running from build\bin or `wails dev`.
func BinDir() string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, "bin"), filepath.Join(dir, "..", "..", "bin"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "bin"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "sing-box.exe")); err == nil {
			return filepath.Clean(c)
		}
	}
	if len(candidates) > 0 {
		return filepath.Clean(candidates[0])
	}
	return "bin"
}

// Missing returns the required files absent from dir.
func Missing(dir string) []string {
	var out []string
	for _, f := range Required {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			out = append(out, f)
		}
	}
	return out
}

// Proc is a supervised child process whose output is forwarded line by line.
type Proc struct {
	Name string
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	mu   sync.Mutex
}

// Start launches exe with args. onLine receives every stdout/stderr line;
// onExit fires once if the process ends on its own (not via Stop).
func Start(name, exe string, args []string, onLine func(string), onExit func(error)) (*Proc, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	p := &Proc{Name: name, cmd: cmd, done: make(chan struct{})}

	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			onLine(fmt.Sprintf("[%s] %s", name, sc.Text()))
		}
	}()
	go func() {
		err := cmd.Wait()
		pw.Close()
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
		if onExit != nil {
			onExit(err)
		}
	}()
	return p, nil
}

// Running reports whether the process is still alive.
func (p *Proc) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Wait blocks up to d for the process to exit; returns its error if it exited.
func (p *Proc) Wait(d time.Duration) (exited bool, err error) {
	select {
	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()
		return true, p.err
	case <-time.After(d):
		return false, nil
	}
}

// Stop kills the process and waits for it to exit.
func (p *Proc) Stop() {
	if p == nil || !p.Running() {
		return
	}
	_ = p.cmd.Process.Kill()
	<-p.done
}
