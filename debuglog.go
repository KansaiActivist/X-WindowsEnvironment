//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var debugLogFile *os.File

// initDebugLog はコンソールの無いGUIアプリでも状況を追えるように、
// %LocalAppData%\XBrowser\debug.log に簡単なログを残す。
func initDebugLog(baseDir string) {
	_ = os.MkdirAll(baseDir, 0o755)
	f, err := os.OpenFile(filepath.Join(baseDir, "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		debugLogFile = f
	}
}

func debugLog(format string, args ...interface{}) {
	if debugLogFile == nil {
		return
	}
	fmt.Fprintf(debugLogFile, "[%s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
