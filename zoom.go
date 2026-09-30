//go:build windows

package main

import (
	"math"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

var zoomLevels = []float64{0.25, 0.33, 0.5, 0.67, 0.75, 0.8, 0.9, 1.0, 1.1, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 4.0, 5.0}

func nearestZoomIndex(z float64) int {
	best, bestD := 0, math.MaxFloat64
	for i, v := range zoomLevels {
		if d := math.Abs(v - z); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// setControllerZoom は ICoreWebView2Controller::put_ZoomFactor を直接呼ぶ。
// (go-webview2 側に公開メソッドが無いため、vtblの並び順で呼び出している)
// vtbl: 0-2 IUnknown, 3 get_IsVisible, 4 put_IsVisible, 5 get_Bounds, 6 put_Bounds,
//
//	7 get_ZoomFactor, 8 put_ZoomFactor
func setControllerZoom(c *edge.ICoreWebView2Controller, z float64) {
	if c == nil {
		return
	}
	vtbl := *(**[16]uintptr)(unsafe.Pointer(c))
	syscall.SyscallN(vtbl[8], uintptr(unsafe.Pointer(c)), uintptr(math.Float64bits(z)))
}

func getControllerZoom(c *edge.ICoreWebView2Controller) float64 {
	if c == nil {
		return 0
	}
	vtbl := *(**[16]uintptr)(unsafe.Pointer(c))
	var z float64
	syscall.SyscallN(vtbl[7], uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(&z)))
	return z
}

// applyZoomTo は現在の拡大率をそのWebViewに適用する。
func applyZoomTo(ch *edge.Chromium) {
	if ch == nil {
		return
	}
	c := ch.GetController()
	setControllerZoom(c, cfg.Zoom)
	if got := getControllerZoom(c); math.Abs(got-cfg.Zoom) > 0.01 {
		debugLog("ズーム適用が反映されていません want=%.2f got=%.2f", cfg.Zoom, got)
	}
}

func (m *TabManager) applyZoomAll() {
	for _, t := range m.tabs {
		if t.ready {
			applyZoomTo(t.chromium)
		}
	}
}

func (m *TabManager) setZoom(z float64) {
	if z < 0.25 {
		z = 0.25
	}
	if z > 5 {
		z = 5
	}
	cfg.Zoom = z
	m.applyZoomAll()
	saveConfig()
	if app != nil {
		invalidateBar()
	}
}

// ZoomStep は拡大(+1)/縮小(-1)を1段階行う。
func (m *TabManager) ZoomStep(dir int) {
	i := nearestZoomIndex(cfg.Zoom) + dir
	if i < 0 {
		i = 0
	}
	if i >= len(zoomLevels) {
		i = len(zoomLevels) - 1
	}
	m.setZoom(zoomLevels[i])
}

func (m *TabManager) ZoomReset() { m.setZoom(1.0) }

// applyNotificationSetting は現在の設定(ON/OFF)を1つのタブへ適用する。
// OFFのときはWebView2の権限要求を常にDenyへ、ONのときはAllowへ即答させる
// (Xの通知許可ダイアログ自体を出さずに、設定どおりの挙動にする)。
func applyNotificationSetting(ch *edge.Chromium) {
	state := edge.CoreWebView2PermissionStateAllow
	if cfg.Notifications == "off" {
		state = edge.CoreWebView2PermissionStateDeny
	}
	ch.SetPermission(edge.CoreWebView2PermissionKindNotifications, state)
}

// applyNotificationSettingAll は開いている全タブへ即時反映する(設定メニューから呼ぶ)。
func (m *TabManager) applyNotificationSettingAll() {
	for _, t := range m.tabs {
		if t.ready {
			applyNotificationSetting(t.chromium)
		}
	}
}
