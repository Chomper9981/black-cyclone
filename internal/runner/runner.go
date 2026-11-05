// Package runner starts and stops the helper process (sing-box).
package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Exe returns the file name of the helper binary called name on this OS
// ("sing-box" -> "sing-box.exe" on Windows).
func Exe(name string) string { return name + exeSuffix }

// GracefulTimeout is how long Stop waits after the polite interrupt before killing.
var GracefulTimeout = 3 * time.Second

// BinDir finds the folder holding the helper binaries: next to the exe (installed build),
// or the project's bin/ when running from build/bin or `wails dev`.
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
		if _, err := os.Stat(filepath.Join(c, Exe("sing-box"))); err == nil {
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

// live holds the PIDs of children started by Start that have not exited yet,
// so KillStale can tell them apart from leftovers.
var live sync.Map

func isLive(pid int) bool { _, ok := live.Load(pid); return ok }

// Proc is a supervised child process whose output is forwarded line by line.
type Proc struct {
	Name     string
	cmd      *exec.Cmd
	done     chan struct{}
	err      error
	mu       sync.Mutex
	stopping bool
	stopOnce sync.Once
	graceful bool
}

// Start launches exe with args. onLine receives every stdout/stderr line;
// onExit fires once if the process ends on its own (not via Stop).
//
// The child is tied to our lifetime (it dies if we crash or are killed) and
// set up so Stop can ask it to exit cleanly; see prepare / afterStart for how
// each OS does that.
func Start(name, exe string, args []string, onLine func(string), onExit func(error)) (*Proc, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	prepare(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	if err := afterStart(name, cmd, onLine); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		pw.Close()
		return nil, fmt.Errorf("start %s: %w", name, err)
	}
	p := &Proc{Name: name, cmd: cmd, done: make(chan struct{})}
	pid := cmd.Process.Pid
	live.Store(pid, struct{}{})

	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			if onLine != nil {
				onLine(fmt.Sprintf("[%s] %s", name, sc.Text()))
			}
		}
		_, _ = io.Copy(io.Discard, pr) // never block the writer if the scanner bails
	}()
	go func() {
		err := cmd.Wait()
		pw.Close()
		live.Delete(pid)
		p.mu.Lock()
		p.err = err
		stopping := p.stopping
		p.mu.Unlock()
		close(p.done)
		if onExit != nil && !stopping {
			onExit(err)
		}
	}()
	return p, nil
}

// PID returns the child's process id (0 for a nil Proc).
func (p *Proc) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Running reports whether the process is still alive.
func (p *Proc) Running() bool {
	if p == nil {
		return false
	}
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

// Stop asks the process to exit (Ctrl+C on Windows, SIGTERM on Linux), waits up to
// GracefulTimeout, then kills it. It blocks until the process is gone.
// onExit is not called for a process stopped this way. Safe on nil.
func (p *Proc) Stop() { p.StopGraceful() }

// StopGraceful is Stop, reporting whether the process exited on the polite
// interrupt (true) or had to be killed / was already gone (false).
func (p *Proc) StopGraceful() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	p.stopping = true
	p.mu.Unlock()
	p.stopOnce.Do(func() {
		if !p.Running() {
			return
		}
		if err := interrupt(p.PID(), p.done); err == nil {
			select {
			case <-p.done:
				p.graceful = true
				return
			case <-time.After(GracefulTimeout):
			}
		}
		_ = p.cmd.Process.Kill()
	})
	<-p.done
	return p.graceful
}
