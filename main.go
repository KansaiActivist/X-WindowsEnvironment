//go:build windows

// xbrowser: X (旧Twitter) 専用の軽量デスクトップブラウザ。
// WebView2 (システムのEdgeランタイムを共有) を使うため、Chromiumを
// まるごと同梱するElectron系アプリよりメモリ消費が小さい。
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
	// WebView2 / COMを使うのでメインスレッドで一度初期化しておく。
	procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
}

func main() {
	// Win32のウィンドウメッセージループとCOM(WebView2)はどちらも
	// 「作成したのと同じOSスレッド」で動く必要がある。Goはgoroutineを
	// 別のOSスレッドへ移動させることがあるため、最初に固定しておく。
	// これを忘れるとWebView2の初期化やメッセージ配送が不安定になり、
	// ページが表示されない/操作が効かないといった症状につながる。
	runtime.LockOSThread()

	if !ensureSingleInstance() {
		// すでに起動している場合は、新しいプロセスは何もせず終了する。
		// (WebView2の同じプロファイルフォルダを2プロセスで取り合うと
		//  片方または両方が固まって画面が真っ黒になることがあるため)
		return
	}

	coInitialize()
	initGdiplus()
	loadConfig() // ウィンドウ作成前に読み込む(背景色などで使う)

	// 高DPI環境でも文字がぼやけないようにする(簡易版)。
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
		HbrBackground: 0, // 背景はWM_PAINTで自前描画するので不要
		LpszClassName: syscall.StringToUTF16Ptr(className),
		HIconSm:       appIconSmall,
	}
	if atom := win.RegisterClassEx(&wc); atom == 0 {
		os.Exit(1)
	}

	registerTabContainerClass(hInstance)

	style := uint32(win.WS_POPUP | win.WS_THICKFRAME | win.WS_MINIMIZEBOX | win.WS_MAXIMIZEBOX | win.WS_CLIPCHILDREN | win.WS_CLIPSIBLINGS)

	// WS_POPUPウィンドウはCW_USEDEFAULTの座標指定が信頼できない(想定外の位置に
	// 出ることがある)ため、現在のモニターのワークエリアを元に、中央寄せした
	// 適当な初期サイズを自分で計算する。
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

	// 先にウィンドウを表示してからWebView2を作る(非表示の親の下で作ると
	// 描画されないままになることがあるため)。
	win.ShowWindow(hwnd, win.SW_SHOW)
	win.UpdateWindow(hwnd)

	// タスクバー/Alt+Tabのアイコンも、ウィンドウに明示的に設定したものが使われる。
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

// initialWindowRect は現在のカーソルがあるモニターのワークエリアを基準に、
// 画面の85%程度のサイズで中央寄せしたウィンドウ矩形を計算する。
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

// loadAppIcons はexe自身に埋め込まれたアイコンリソース(resource.syso /
// favicon.syso などで埋め込んだもの、リソースID 1を想定)を読み込む。
// 埋め込みが無い/読み込めない場合はWindows既定のアイコンにフォールバックする。
//
// 以前はここで win.LoadIcon(0, ...) と「0」(システム側)を指定していたため、
// .sysoでexeにアイコンを埋め込んでも、ウィンドウ/タスクバーには常にWindowsの
// 汎用アイコンが表示されていた。自分のモジュール(hInstance)からリソースID 1を
// 読みに行くようにすることで、埋め込んだアイコンがそのまま使われるようにする。
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
