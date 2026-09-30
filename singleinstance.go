//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const errorAlreadyExists = 183

var (
	kernel32b        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW = kernel32b.NewProc("CreateMutexW")
)

// ensureSingleInstance は同時に2つ目のxbrowser.exeが起動するのを防ぐ。
// WebView2は同じユーザーデータフォルダを複数プロセスで同時に使えないため、
// 二重起動すると片方(または両方)が固まって真っ黒な画面になることがある。
// すでに起動していた場合は、そのウィンドウを前面に出してtrueを返さない。
func ensureSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Local\\XBrowserSingleInstanceMutex")
	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		// Mutex作成に失敗した場合は、多重起動チェックを諦めてそのまま起動する。
		return true
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		classPtr, _ := syscall.UTF16PtrFromString(className)
		existing := win.FindWindow(classPtr, nil)
		if existing != 0 {
			win.ShowWindow(existing, win.SW_RESTORE)
			win.SetForegroundWindow(existing)
		}
		return false
	}
	return true
}
