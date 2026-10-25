package rcvcore

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const maxLogLines = 400

var (
	logMu    sync.Mutex
	logLines []string
)

func logf(format string, args ...any) {
	line := time.Now().Format("15:04:05 ") + fmt.Sprintf(format, args...)
	logMu.Lock()
	defer logMu.Unlock()
	logLines = append(logLines, line)
	if len(logLines) > maxLogLines {
		logLines = append([]string(nil), logLines[len(logLines)-maxLogLines:]...)
	}
}

// Log adds a line from the app side to the shared log.
func Log(line string) { logf("%s", line) }

// Logs returns the recent log lines, oldest first, newline-separated.
func Logs() string {
	logMu.Lock()
	defer logMu.Unlock()
	return strings.Join(logLines, "\n")
}

func ClearLogs() {
	logMu.Lock()
	defer logMu.Unlock()
	logLines = nil
}
