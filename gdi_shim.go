//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)


var (
	gdi32   = syscall.NewLazyDLL("gdi32.dll")
	user32b = syscall.NewLazyDLL("user32.dll")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procFillRect         = user32b.NewProc("FillRect")
	procDrawTextW        = user32b.NewProc("DrawTextW")
	procMonitorFromPoint = user32b.NewProc("MonitorFromPoint")
	procGetWindowTextW   = user32b.NewProc("GetWindowTextW")
	procSetWindowTextW   = user32b.NewProc("SetWindowTextW")
)

func getWindowText(hwnd win.HWND, buf []uint16) int32 {
	r, _, _ := procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return int32(r)
}

func setWindowText(hwnd win.HWND, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(p)))
}


func monitorFromPoint(pt win.POINT, flags uint32) win.HMONITOR {
	packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
	r, _, _ := procMonitorFromPoint.Call(uintptr(packed), uintptr(flags))
	return win.HMONITOR(r)
}

func createSolidBrush(col win.COLORREF) win.HBRUSH {
	r, _, _ := procCreateSolidBrush.Call(uintptr(col))
	return win.HBRUSH(r)
}

func winFillRect(hdc win.HDC, rect *win.RECT, brush win.HBRUSH) {
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(rect)), uintptr(brush))
}

func winDrawText(hdc win.HDC, text *uint16, count int32, rect *win.RECT, format uint32) {
	procDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(text)), uintptr(count), uintptr(unsafe.Pointer(rect)), uintptr(format))
}
