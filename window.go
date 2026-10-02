//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	titleBarH    = 46 
	navBtnW      = 38 
	tabMinW      = 96
	tabMaxW      = 240
	tabGap       = 4
	newTabBtnW   = 38
	winBtnW      = 48 
	zoomBtnW     = 30 
	zoomLabelW   = 56 
	zoomClusterW = zoomBtnW*2 + zoomLabelW
	resizeGrip   = 6 

	sizeMinimized    = 1  
	smCXPaddedBorder = 92 
)

type ncCalcSizeParams struct {
	Rgrc  [3]win.RECT
	Lppos uintptr
}

type hitZone int

const (
	zoneNone hitZone = iota
	zoneBack
	zoneForward
	zoneReload
	zoneNewTab
	zoneMinimize
	zoneMaximize
	zoneClose
	zoneTab
	zoneTabClose
	zoneZoomOut
	zoneZoomLabel
	zoneZoomIn
)

type ChromeWindow struct {
	hwnd     win.HWND
	tabs     *TabManager
	hovered  hitZone
	hoverTab int
	tracking bool
}

var app *ChromeWindow

func initChromeWindow(hwnd win.HWND) {
	app = &ChromeWindow{hwnd: hwnd, hoverTab: -1}
	app.tabs = newTabManager(hwnd)
	app.layout()
	app.tabs.AddTab(homeURL, "")
	app.layout()
}


func (c *ChromeWindow) contentRect() win.RECT {
	var rc win.RECT
	win.GetClientRect(c.hwnd, &rc)
	rc.Top = titleBarH
	return rc
}

func (c *ChromeWindow) layout() {
	if c.tabs != nil {
		c.tabs.Resize(c.contentRect())
	}
	win.InvalidateRect(c.hwnd, nil, false)
}

func (c *ChromeWindow) tabRects() []win.RECT {
	var client win.RECT
	win.GetClientRect(c.hwnd, &client)
	width := int32(client.Right - client.Left)

	left := int32(navBtnW*3 + 6)
	right := width - int32(winBtnW*3+zoomClusterW+14)
	avail := right - left - newTabBtnW
	if avail < 0 {
		avail = 0
	}

	n := int32(len(c.tabs.tabs))
	if n == 0 {
		return nil
	}
	tw := int32(tabMaxW)
	if avail/n < int32(tabMaxW) {
		tw = avail / n
	}
	if tw < int32(tabMinW) {
		tw = int32(tabMinW)
	}

	rects := make([]win.RECT, n)
	x := left
	for i := int32(0); i < n; i++ {
		rects[i] = win.RECT{Left: x, Top: 0, Right: x + tw - tabGap, Bottom: titleBarH}
		x += tw
	}
	return rects
}

func (c *ChromeWindow) newTabBtnRect() win.RECT {
	rects := c.tabRects()
	x := int32(navBtnW*3 + 6)
	if len(rects) > 0 {
		x = rects[len(rects)-1].Right + tabGap
	}
	return win.RECT{Left: x, Top: 0, Right: x + newTabBtnW, Bottom: titleBarH}
}

func (c *ChromeWindow) winButtonRect(which hitZone) win.RECT {
	var client win.RECT
	win.GetClientRect(c.hwnd, &client)
	width := int32(client.Right - client.Left)
	switch which {
	case zoneClose:
		return win.RECT{Left: width - winBtnW, Top: 0, Right: width, Bottom: titleBarH}
	case zoneMaximize:
		return win.RECT{Left: width - winBtnW*2, Top: 0, Right: width - winBtnW, Bottom: titleBarH}
	case zoneMinimize:
		return win.RECT{Left: width - winBtnW*3, Top: 0, Right: width - winBtnW*2, Bottom: titleBarH}
	}
	return win.RECT{}
}

func (c *ChromeWindow) navButtonRect(which hitZone) win.RECT {
	switch which {
	case zoneBack:
		return win.RECT{Left: 0, Top: 0, Right: navBtnW, Bottom: titleBarH}
	case zoneForward:
		return win.RECT{Left: navBtnW, Top: 0, Right: navBtnW * 2, Bottom: titleBarH}
	case zoneReload:
		return win.RECT{Left: navBtnW * 2, Top: 0, Right: navBtnW * 3, Bottom: titleBarH}
	}
	return win.RECT{}
}

func tabVisualRect(slot win.RECT) win.RECT {
	return win.RECT{Left: slot.Left, Top: slot.Top + 8, Right: slot.Right, Bottom: slot.Bottom - 4}
}

func tabCloseRect(slot win.RECT) win.RECT {
	vr := tabVisualRect(slot)
	const sz = 24
	top := vr.Top + (vr.Bottom-vr.Top-sz)/2
	return win.RECT{Left: vr.Right - 8 - sz, Top: top, Right: vr.Right - 8, Bottom: top + sz}
}

func (c *ChromeWindow) zoomRect(which hitZone) win.RECT {
	var client win.RECT
	win.GetClientRect(c.hwnd, &client)
	width := client.Right - client.Left
	base := width - int32(winBtnW*3+zoomClusterW+6)
	top, bottom := int32(9), int32(titleBarH-9)
	switch which {
	case zoneZoomOut:
		return win.RECT{Left: base, Top: top, Right: base + zoomBtnW, Bottom: bottom}
	case zoneZoomLabel:
		return win.RECT{Left: base + zoomBtnW, Top: top, Right: base + zoomBtnW + zoomLabelW, Bottom: bottom}
	case zoneZoomIn:
		return win.RECT{Left: base + zoomBtnW + zoomLabelW, Top: top, Right: base + zoomClusterW, Bottom: bottom}
	}
	return win.RECT{}
}

func invalidateBar() {
	r := win.RECT{Left: 0, Top: 0, Right: 32000, Bottom: titleBarH}
	win.InvalidateRect(app.hwnd, &r, false)
}

func ptInRect(r win.RECT, x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (c *ChromeWindow) hitTest(x, y int32) (hitZone, int) {
	if y >= titleBarH {
		return zoneNone, -1
	}
	if ptInRect(c.navButtonRect(zoneBack), x, y) {
		return zoneBack, -1
	}
	if ptInRect(c.navButtonRect(zoneForward), x, y) {
		return zoneForward, -1
	}
	if ptInRect(c.navButtonRect(zoneReload), x, y) {
		return zoneReload, -1
	}
	if ptInRect(c.newTabBtnRect(), x, y) {
		return zoneNewTab, -1
	}
	for _, z := range []hitZone{zoneZoomOut, zoneZoomLabel, zoneZoomIn} {
		if ptInRect(c.zoomRect(z), x, y) {
			return z, -1
		}
	}
	if ptInRect(c.winButtonRect(zoneMinimize), x, y) {
		return zoneMinimize, -1
	}
	if ptInRect(c.winButtonRect(zoneMaximize), x, y) {
		return zoneMaximize, -1
	}
	if ptInRect(c.winButtonRect(zoneClose), x, y) {
		return zoneClose, -1
	}
	for i, r := range c.tabRects() {
		if ptInRect(r, x, y) {
			if ptInRect(tabCloseRect(r), x, y) {
				return zoneTabClose, i
			}
			return zoneTab, i
		}
	}
	return zoneNone, -1
}

func isMaximized(hwnd win.HWND) bool {
	return win.IsZoomed(hwnd)
}

func (c *ChromeWindow) toggleMaximize() {
	if isMaximized(c.hwnd) {
		win.ShowWindow(c.hwnd, win.SW_RESTORE)
	} else {
		win.ShowWindow(c.hwnd, win.SW_MAXIMIZE)
	}
}

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_NCCALCSIZE:
		if wParam != 0 && isMaximized(hwnd) {
			maximizedClientRect(hwnd, lParam)
		}
		return 0
	case win.WM_GETMINMAXINFO:
		mmi := (*win.MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize.X = 480
		mmi.PtMinTrackSize.Y = 360
		if mon := win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST); mon != 0 {
			var mi win.MONITORINFO
			mi.CbSize = uint32(unsafe.Sizeof(mi))
			if win.GetMonitorInfo(mon, &mi) {
				work, monRc := mi.RcWork, mi.RcMonitor
				mmi.PtMaxPosition.X = work.Left - monRc.Left
				mmi.PtMaxPosition.Y = work.Top - monRc.Top
				mmi.PtMaxSize.X = work.Right - work.Left
				mmi.PtMaxSize.Y = work.Bottom - work.Top
			}
		}
		return 0
	case win.WM_NCHITTEST:
		return ncHitTest(hwnd, lParam)
	case win.WM_NCACTIVATE:
		return win.DefWindowProc(hwnd, msg, wParam, ^uintptr(0))
	case win.WM_NCPAINT:
		return 0
	case win.WM_ERASEBKGND:
		var rc win.RECT
		win.GetClientRect(hwnd, &rc)
		brush := createSolidBrush(palette.bar)
		winFillRect(win.HDC(wParam), &rc, brush)
		win.DeleteObject(win.HGDIOBJ(brush))
		return 1
	}

	if app == nil || app.hwnd != hwnd {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}

	switch msg {
	case win.WM_ACTIVATE, win.WM_ACTIVATEAPP, win.WM_SETFOCUS, win.WM_DISPLAYCHANGE, win.WM_WINDOWPOSCHANGED:
		r := win.DefWindowProc(hwnd, msg, wParam, lParam)
		repaintBar()
		return r

	case win.WM_RBUTTONUP:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		if y < titleBarH {
			showContextMenu(hwnd, x, y)
		}
		return 0

	case win.WM_SIZE:
		if wParam == sizeMinimized {
			return 0
		}
		app.layout()
		return 0

	case win.WM_MOVE, win.WM_SHOWWINDOW:
		if app.tabs != nil {
			app.tabs.Refresh()
		}
		return win.DefWindowProc(hwnd, msg, wParam, lParam)

	case win.WM_PAINT:
		paintChrome(hwnd)
		return 0

	case win.WM_LBUTTONDOWN:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		zone, idx := app.hitTest(x, y)
		switch zone {
		case zoneBack:
			app.tabs.Active().Eval("history.back()")
		case zoneForward:
			app.tabs.Active().Eval("history.forward()")
		case zoneReload:
			app.tabs.Active().Eval("location.reload()")
		case zoneNewTab:
			app.tabs.AddTab(homeURL, app.tabs.Active().profile)
			app.layout()
		case zoneTab:
			app.tabs.Switch(idx)
			app.layout()
		case zoneTabClose:
			app.tabs.Close(idx)
			app.layout()
		case zoneZoomOut:
			app.tabs.ZoomStep(-1)
		case zoneZoomIn:
			app.tabs.ZoomStep(1)
		case zoneZoomLabel:
			app.tabs.ZoomReset()
		case zoneMinimize:
			win.ShowWindow(hwnd, win.SW_MINIMIZE)
		case zoneMaximize:
			app.toggleMaximize()
		case zoneClose:
			win.DestroyWindow(hwnd)
		case zoneNone:
			if y < titleBarH {
				win.ReleaseCapture()
				win.PostMessage(hwnd, win.WM_NCLBUTTONDOWN, uintptr(win.HTCAPTION), 0)
			}
		}
		return 0

	case win.WM_LBUTTONDBLCLK:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		zone, _ := app.hitTest(x, y)
		if zone == zoneNone && y < titleBarH {
			app.toggleMaximize()
		}
		return 0

	case win.WM_MOUSEMOVE:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		zone, idx := app.hitTest(x, y)
		if zone != app.hovered || idx != app.hoverTab {
			app.hovered = zone
			app.hoverTab = idx
			win.InvalidateRect(hwnd, nil, false)
		}
		if !app.tracking {
			app.tracking = true
			tme := win.TRACKMOUSEEVENT{
				CbSize:    uint32(unsafe.Sizeof(win.TRACKMOUSEEVENT{})),
				DwFlags:   win.TME_LEAVE,
				HwndTrack: hwnd,
			}
			win.TrackMouseEvent(&tme)
		}
		return 0

	case win.WM_MOUSELEAVE:
		app.tracking = false
		app.hovered = zoneNone
		app.hoverTab = -1
		win.InvalidateRect(hwnd, nil, false)
		return 0

	case win.WM_CLOSE:
		win.DestroyWindow(hwnd)
		return 0

	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}

	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func ncHitTest(hwnd win.HWND, lParam uintptr) uintptr {
	var rc win.RECT
	win.GetWindowRect(hwnd, &rc)
	x := int32(int16(lParam & 0xffff))
	y := int32(int16((lParam >> 16) & 0xffff))

	left := x < rc.Left+resizeGrip
	right := x >= rc.Right-resizeGrip
	top := y < rc.Top+resizeGrip
	bottom := y >= rc.Bottom-resizeGrip

	switch {
	case top && left:
		return uintptr(win.HTTOPLEFT)
	case top && right:
		return uintptr(win.HTTOPRIGHT)
	case bottom && left:
		return uintptr(win.HTBOTTOMLEFT)
	case bottom && right:
		return uintptr(win.HTBOTTOMRIGHT)
	case left:
		return uintptr(win.HTLEFT)
	case right:
		return uintptr(win.HTRIGHT)
	case top:
		return uintptr(win.HTTOP)
	case bottom:
		return uintptr(win.HTBOTTOM)
	}
	return uintptr(win.HTCLIENT)
}

func maximizedClientRect(hwnd win.HWND, lParam uintptr) {
	mon := win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST)
	if mon == 0 {
		return
	}
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(mon, &mi) {
		return
	}
	params := (*ncCalcSizeParams)(unsafe.Pointer(lParam))
	params.Rgrc[0] = mi.RcWork
}

func repaintBar() {
	r := win.RECT{Left: 0, Top: 0, Right: 32000, Bottom: titleBarH}
	win.RedrawWindow(app.hwnd, &r, 0, win.RDW_INVALIDATE|win.RDW_UPDATENOW)
}

func mustUTF16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}
