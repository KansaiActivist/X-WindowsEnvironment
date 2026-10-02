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


func ensureSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Local\\XBrowserSingleInstanceMutex")
	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
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
