//go:build windows


package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	homeURL   = "https://x.com/home"
	className = "XBrowserMainWindowClass"
)

var (
	ole32                            = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx               = ole32.NewProc("CoInitializeEx")
	COINIT_APARTMENTTHREADED uintptr = 0x2
)

func coInitialize() {
	procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
}

func main() {
	runtime.LockOSThread()

	if !ensureSingleInstance() {
		return
	}

	coInitialize()
	initGdiplus()
	loadConfig() 

	user32 := syscall.NewLazyDLL("user32.dll")
	if p := user32.NewProc("SetProcessDPIAware"); p.Find() == nil {
		p.Call()
	}

	hInstance := win.GetModuleHandle(nil)
	appIconLarge, appIconSmall := loadAppIcons(hInstance)

	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HIcon:         appIconLarge,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: 0, 
		LpszClassName: syscall.StringToUTF16Ptr(className),
		HIconSm:       appIconSmall,
	}
	if atom := win.RegisterClassEx(&wc); atom == 0 {
		os.Exit(1)
	}

	registerTabContainerClass(hInstance)

	style := uint32(win.WS_POPUP | win.WS_THICKFRAME | win.WS_MINIMIZEBOX | win.WS_MAXIMIZEBOX | win.WS_CLIPCHILDREN | win.WS_CLIPSIBLINGS)

	x, y, cw, ch := initialWindowRect()

	hwnd := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr(className),
		syscall.StringToUTF16Ptr("X"),
		style,
		x, y, cw, ch,
		0, 0, hInstance, nil,
	)
	if hwnd == 0 {
		os.Exit(1)
	}

	win.ShowWindow(hwnd, win.SW_SHOW)
	win.UpdateWindow(hwnd)

	win.SendMessage(hwnd, win.WM_SETICON, 1, uintptr(appIconLarge))
	win.SendMessage(hwnd, win.WM_SETICON, 0, uintptr(appIconSmall))

	initChromeWindow(hwnd)
	app.tabs.Refresh()

	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) != 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func initialWindowRect() (x, y, w, h int32) {
	var pt win.POINT
	win.GetCursorPos(&pt)
	mon := monitorFromPoint(pt, win.MONITOR_DEFAULTTOPRIMARY)
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if mon == 0 || !win.GetMonitorInfo(mon, &mi) {
		return win.CW_USEDEFAULT, win.CW_USEDEFAULT, 1280, 820
	}
	work := mi.RcWork
	ww, wh := work.Right-work.Left, work.Bottom-work.Top
	w = ww * 85 / 100
	h = wh * 85 / 100
	if w < 480 {
		w = ww
	}
	if h < 360 {
		h = wh
	}
	x = work.Left + (ww-w)/2
	y = work.Top + (wh-h)/2
	return
}

func loadAppIcons(hInstance win.HINSTANCE) (big win.HICON, small win.HICON) {
	name := win.MAKEINTRESOURCE(1)
	big = win.HICON(win.LoadImage(hInstance, name, win.IMAGE_ICON,
		win.GetSystemMetrics(win.SM_CXICON), win.GetSystemMetrics(win.SM_CYICON), 0))
	small = win.HICON(win.LoadImage(hInstance, name, win.IMAGE_ICON,
		win.GetSystemMetrics(win.SM_CXSMICON), win.GetSystemMetrics(win.SM_CYSMICON), 0))
	if big == 0 {
		big = win.LoadIcon(0, win.MAKEINTRESOURCE(win.IDI_APPLICATION))
	}
	if small == 0 {
		small = big
	}
	return
}
