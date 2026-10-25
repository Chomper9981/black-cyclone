package autostart

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

const testTask = "Black Cyclone autostart test"

// TestEnableDisable registers a real task under a test name (so a user's real
// entry is untouched). Elevated: HighestAvailable, exactly like Enable.
// Not elevated: LeastPrivilege, which still validates the XML and the flow.
func TestEnableDisable(t *testing.T) {
	level := "HighestAvailable"
	if !windows.GetCurrentProcessToken().IsElevated() {
		level = "LeastPrivilege"
		t.Log("not elevated: registering with LeastPrivilege")
	}
	_ = disable(testTask)
	if exists(testTask) {
		t.Fatal("test task exists before Enable")
	}
	exe := `C:\Program Files\Black Cyclone\BlackCyclone.exe`
	if err := enable(testTask, level, exe, "--minimized", "a b&c"); err != nil {
		if strings.Contains(err.Error(), "Access is denied") || strings.Contains(err.Error(), "0x80070005") {
			t.Skipf("no permission to register tasks: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = disable(testTask) })
	if !exists(testTask) {
		t.Fatal("task not found after Enable")
	}
	out, err := schtasks("/Query", "/TN", testTask, "/XML")
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{
		"<Command>" + exe + "</Command>",
		"<Arguments>--minimized \"a b&amp;c\"</Arguments>",
		"<LogonType>InteractiveToken</LogonType>",
		"<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>",
		"<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>",
	}
	if level == "HighestAvailable" { // LeastPrivilege is the default and omitted on export
		wants = append(wants, "<RunLevel>HighestAvailable</RunLevel>")
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("task XML missing %q\n%s", want, out)
		}
	}
	// Re-enable must replace, not fail.
	if err := enable(testTask, level, exe); err != nil {
		t.Fatal(err)
	}
	if err := disable(testTask); err != nil {
		t.Fatal(err)
	}
	if exists(testTask) {
		t.Fatal("task still present after Disable")
	}
	if err := disable(testTask); err != nil {
		t.Fatalf("Disable of missing task: %v", err)
	}
}
