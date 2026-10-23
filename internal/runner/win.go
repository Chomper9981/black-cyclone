package runner

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole         = kernel32.NewProc("AttachConsole")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
	procIsProcessInJob        = kernel32.NewProc("IsProcessInJob")
)

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS (DWORD)-1

// ---- Job Object -----------------------------------------------------------

var (
	jobOnce   sync.Once
	jobHandle windows.Handle
	jobErr    error
)

// job returns the process-wide kill-on-close Job Object, creating it once.
// The handle is intentionally never closed: when our process exits (cleanly,
// by crash or by TerminateProcess) the kernel closes it and kills every child.
func job() (windows.Handle, error) {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = fmt.Errorf("CreateJobObject: %w", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			jobErr = fmt.Errorf("SetInformationJobObject: %w", err)
			return
		}
		jobHandle = h
	})
	return jobHandle, jobErr
}

func assignToJob(pid uint32) error {
	j, err := job()
	if err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("OpenProcess: %w", err)
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(j, h); err != nil {
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return nil
}

// inOurJob reports whether the process handle belongs to our Job Object.
func inOurJob(h windows.Handle) bool {
	if jobHandle == 0 {
		return false
	}
	var res int32
	r, _, _ := procIsProcessInJob.Call(uintptr(h), uintptr(jobHandle), uintptr(unsafe.Pointer(&res)))
	return r != 0 && res != 0
}

// resumeProcess resumes every thread of a process created with CREATE_SUSPENDED.
func resumeProcess(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)
	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	resumed := 0
	for err = windows.Thread32First(snap, &te); err == nil; err = windows.Thread32Next(snap, &te) {
		if te.OwnerProcessID != pid {
			continue
		}
		th, oerr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, te.ThreadID)
		if oerr != nil {
			return fmt.Errorf("OpenThread: %w", oerr)
		}
		_, rerr := windows.ResumeThread(th)
		windows.CloseHandle(th)
		if rerr != nil {
			return fmt.Errorf("ResumeThread: %w", rerr)
		}
		resumed++
	}
	if resumed == 0 {
		return errors.New("no threads found")
	}
	return nil
}

// ---- Graceful stop (Ctrl+C / Ctrl+Break) ----------------------------------

var (
	consoleMu sync.Mutex
	ctrlSeen  = make(chan uintptr, 16)
	// swallowCtrl is installed while we are attached to a child's console so
	// the event we broadcast to that console does not reach Go's handler (which,
	// without signal.Notify, lets Windows terminate *us*). It also reports each
	// delivery so we stay attached until it has arrived: a delivery that lands
	// after FreeConsole bypasses the handler list and kills the process.
	// Created once: callbacks are a finite resource.
	swallowCtrl = syscall.NewCallback(func(ctrlType uintptr) uintptr {
		select {
		case ctrlSeen <- ctrlType:
		default:
		}
		return 1
	})
	inheritOnce sync.Once
)

// ctrlBreakDelay is how long sendCtrlC waits after Ctrl+C before also sending Ctrl+Break.
const ctrlBreakDelay = 500 * time.Millisecond

// processCtrlC clears this process's "ignore Ctrl+C" flag (set if our parent
// started us with CREATE_NEW_PROCESS_GROUP), because children inherit it and
// would then ignore the Ctrl+C sent by Stop. Harmless for a GUI app with no console.
func processCtrlC() {
	inheritOnce.Do(func() { procSetConsoleCtrlHandler.Call(0, 0) })
}

func waitCtrlSeen(typ uintptr, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case got := <-ctrlSeen:
			if got == typ {
				return true
			}
		case <-t.C:
			return false
		}
	}
}

// sendCtrlC asks the console process pid to exit. Each child owns a hidden
// console (CREATE_NO_WINDOW), so broadcasting to group 0 of that console reaches
// only the child (and its descendants) plus us, and we swallow ours.
// Ctrl+C is sent first (C programs' signal(SIGINT) only sees this one); if the
// child is still running after ctrlBreakDelay, Ctrl+Break follows (it cannot be
// ignored via the inheritable flag; Go programs such as sing-box treat both as
// os.Interrupt).
//
// Attaching to a console is process-wide, hence the global mutex. A Wails GUI
// process normally has no console; if we do have one (e.g. `go test` in a
// terminal) it is detached for the duration and re-attached to the parent's after.
func sendCtrlC(pid uint32, done <-chan struct{}) error {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	hadConsole := false
	if r, _, e := procAttachConsole.Call(uintptr(pid)); r == 0 {
		if !errors.Is(e, windows.ERROR_ACCESS_DENIED) { // ACCESS_DENIED = we already have one
			return fmt.Errorf("AttachConsole(%d): %w", pid, e)
		}
		hadConsole = true
		procFreeConsole.Call()
		if r, _, e := procAttachConsole.Call(uintptr(pid)); r == 0 {
			procAttachConsole.Call(uintptr(attachParentProcess))
			return fmt.Errorf("AttachConsole(%d): %w", pid, e)
		}
	}
	defer func() {
		procFreeConsole.Call()
		if hadConsole {
			procAttachConsole.Call(uintptr(attachParentProcess))
		}
		time.Sleep(20 * time.Millisecond)
		procSetConsoleCtrlHandler.Call(swallowCtrl, 0)
	}()
	if r, _, e := procSetConsoleCtrlHandler.Call(swallowCtrl, 1); r == 0 {
		return fmt.Errorf("SetConsoleCtrlHandler: %w", e)
	}
	for len(ctrlSeen) > 0 {
		<-ctrlSeen
	}

	start := time.Now()
	sent := false
	if windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0) == nil {
		sent = true
		// Not seen if we ignore Ctrl+C ourselves; then nothing arrives later either.
		waitCtrlSeen(windows.CTRL_C_EVENT, ctrlBreakDelay)
	}
	select {
	case <-done:
		return nil
	case <-time.After(ctrlBreakDelay - time.Since(start)):
	}
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, 0)
	if err == nil {
		sent = true
		if !waitCtrlSeen(windows.CTRL_BREAK_EVENT, 2*time.Second) {
			// Never detach before our copy arrives (see swallowCtrl). If it
			// hasn't come in 2s something is off; give up on graceful.
			return errors.New("Ctrl+Break delivery not confirmed")
		}
	}
	if !sent {
		return fmt.Errorf("GenerateConsoleCtrlEvent: %w", err)
	}
	return nil
}

// ---- Stale process cleanup ------------------------------------------------

// staleNames are the helper images KillStale looks for.
var staleNames = []string{"sing-box.exe", "goodbyedpi.exe"}

// KillStale terminates leftover sing-box.exe / goodbyedpi.exe processes whose
// image lives inside binDir (e.g. orphaned by an older, crashed version).
// Processes belonging to this app's Job Object are left alone. It returns the
// number of processes killed; err aggregates per-process failures.
func KillStale(binDir string) (int, error) {
	want, err := filepath.Abs(binDir)
	if err != nil {
		return 0, err
	}
	want = filepath.Clean(want)
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	self := windows.GetCurrentProcessId()
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	killed := 0
	var errs []error
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if pe.ProcessID == self || !isStaleName(windows.UTF16ToString(pe.ExeFile[:])) {
			continue
		}
		ok, kerr := killIfInDir(pe.ProcessID, want)
		if kerr != nil {
			errs = append(errs, kerr)
		}
		if ok {
			killed++
		}
	}
	return killed, errors.Join(errs...)
}

func isStaleName(n string) bool {
	for _, s := range staleNames {
		if strings.EqualFold(n, s) {
			return true
		}
	}
	return false
}

func killIfInDir(pid uint32, dir string) (bool, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return false, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return false, fmt.Errorf("QueryFullProcessImageName(%d): %w", pid, err)
	}
	img := windows.UTF16ToString(buf[:n])
	if !strings.EqualFold(filepath.Clean(filepath.Dir(img)), dir) || inOurJob(h) {
		return false, nil
	}
	if err := windows.TerminateProcess(h, 1); err != nil {
		return false, fmt.Errorf("TerminateProcess(%d %s): %w", pid, img, err)
	}
	_, _ = windows.WaitForSingleObject(h, 3000)
	return true, nil
}
