package app

import (
	"fmt"
	"os"
	"time"
)

var (
	originalStdout *os.File
	appLogFile     *os.File
)

func saveOriginalStdout() {
	if originalStdout == nil {
		originalStdout = os.Stdout
	}
}

func initLogger(file *os.File) {
	appLogFile = file
}

func logPrintf(format string, a ...any) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	msg := fmt.Sprintf("["+timestamp+"] "+format, a...)

	if originalStdout != nil {
		_, _ = originalStdout.WriteString(msg)
	}
	if appLogFile != nil {
		_, _ = appLogFile.WriteString(msg)
	}
}

func logPrintln(a ...any) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	header := "[" + timestamp + "] "
	msg := fmt.Sprintln(a...)
	fullMsg := header + msg

	if originalStdout != nil {
		_, _ = originalStdout.WriteString(fullMsg)
	}
	if appLogFile != nil {
		_, _ = appLogFile.WriteString(fullMsg)
	}
}
