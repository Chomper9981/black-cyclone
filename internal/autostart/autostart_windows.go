// On Windows the app starts at logon via Task Scheduler.
//
// A Run-key entry cannot launch an app whose manifest requires elevation
// without a UAC prompt (Windows silently skips it), so we create a scheduled
// task with "Run with highest privileges" and an interactive-token logon
// trigger for the current user. Registering requires an elevated caller.
package autostart

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// TaskName is the Task Scheduler entry name.
const TaskName = "Black Cyclone"

// legacyTaskName is the task created by builds released as "RedCyclone VPN".
const legacyTaskName = "RedCyclone VPN"

// Delay before the task launches after logon, so the shell/tray is ready.
const logonDelay = "PT10S"

// Enable registers (or replaces) the logon task running exePath with args.
func Enable(exePath string, args ...string) error {
	_ = disable(legacyTaskName) // would launch the old exe as well
	return enable(TaskName, "HighestAvailable", exePath, args...)
}

// Disable removes the logon task. Removing a task that does not exist is not an error.
func Disable() error {
	_ = disable(legacyTaskName)
	return disable(TaskName)
}

// Enabled reports whether the logon task exists.
func Enabled() bool { return exists(TaskName) }

func enable(name, runLevel, exePath string, args ...string) error {
	exe, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	user, err := currentUser()
	if err != nil {
		return err
	}
	doc, err := taskXML(user, runLevel, exe, args)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "blackcyclone-task-*.xml")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, werr := f.Write(doc)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return err
	}
	_, err = schtasks("/Create", "/TN", name, "/XML", tmp, "/F")
	return err
}

func disable(name string) error {
	if !exists(name) {
		return nil
	}
	_, err := schtasks("/Delete", "/TN", name, "/F")
	return err
}

func exists(name string) bool {
	_, err := schtasks("/Query", "/TN", name)
	return err == nil
}

// schtasks runs schtasks.exe without flashing a console window.
func schtasks(args ...string) (string, error) {
	sys, err := windows.GetSystemDirectory()
	if err != nil {
		sys = `C:\Windows\System32`
	}
	cmd := exec.Command(filepath.Join(sys, "schtasks.exe"), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("schtasks %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// currentUser returns DOMAIN\user for the process token's user.
func currentUser() (string, error) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("token user: %w", err)
	}
	account, domain, _, err := tu.User.Sid.LookupAccount("")
	if err != nil {
		return tu.User.Sid.String(), nil
	}
	if domain == "" {
		return account, nil
	}
	return domain + `\` + account, nil
}

// taskXML builds a Task Scheduler 1.2 definition, UTF-16LE with BOM as schtasks expects.
// Compared with `schtasks /SC ONLOGON` it lets us: scope the trigger to this user
// (not "any user"), force an interactive token (so the UI shows in the user's
// session), and lift the defaults that would stop the app (72h execution time
// limit, battery restrictions, idle stop).
func taskXML(user, runLevel, exe string, args []string) ([]byte, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	argsXML := ""
	if len(args) > 0 {
		argsXML = "\n      <Arguments>" + esc(strings.Join(quoted, " ")) + "</Arguments>"
	}
	s := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Start Black Cyclone at logon</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + esc(user) + `</UserId>
      <Delay>` + logonDelay + `</Delay>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + esc(user) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>` + runLevel + `</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + esc(exe) + `</Command>` + argsXML + `
      <WorkingDirectory>` + esc(filepath.Dir(exe)) + `</WorkingDirectory>
    </Exec>
  </Actions>
</Task>
`
	u := utf16.Encode([]rune(s))
	out := make([]byte, 2+2*len(u))
	out[0], out[1] = 0xFF, 0xFE
	for i, c := range u {
		out[2+2*i] = byte(c)
		out[3+2*i] = byte(c >> 8)
	}
	return out, nil
}
