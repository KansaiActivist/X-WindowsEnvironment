//go:build windows

package main

import (
	"math"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

// GDI+ (アンチエイリアス付き)で角丸を描くための最小ラッパー。
// 初期化に失敗した場合は通常のGDI(RoundRect)にフォールバックする。
var (
	gdiplusDLL           = syscall.NewLazyDLL("gdiplus.dll")
	pGdiplusStartup      = gdiplusDLL.NewProc("GdiplusStartup")
	pGdipCreateFromHDC   = gdiplusDLL.NewProc("GdipCreateFromHDC")
	pGdipDeleteGraphics  = gdiplusDLL.NewProc("GdipDeleteGraphics")
	pGdipSetSmoothing    = gdiplusDLL.NewProc("GdipSetSmoothingMode")
	pGdipCreateSolidFill = gdiplusDLL.NewProc("GdipCreateSolidFill")
	pGdipDeleteBrush     = gdiplusDLL.NewProc("GdipDeleteBrush")
	pGdipCreatePath      = gdiplusDLL.NewProc("GdipCreatePath")
	pGdipDeletePath      = gdiplusDLL.NewProc("GdipDeletePath")
	pGdipAddPathArc      = gdiplusDLL.NewProc("GdipAddPathArc")
	pGdipClosePathFigure = gdiplusDLL.NewProc("GdipClosePathFigure")
	pGdipFillPath        = gdiplusDLL.NewProc("GdipFillPath")

	gdiplusToken uintptr
	gdiplusReady bool
)

func initGdiplus() {
	input := struct {
		Version        uint32
		DebugCallback  uintptr
		SuppressBG     int32
		SuppressCodecs int32
	}{Version: 1}
	r, _, _ := pGdiplusStartup.Call(uintptr(unsafe.Pointer(&gdiplusToken)), uintptr(unsafe.Pointer(&input)), 0)
	gdiplusReady = r == 0
}

func f32(v float32) uintptr { return uintptr(math.Float32bits(v)) }

func argb(c win.COLORREF) uintptr {
	r := uint32(c) & 0xff
	g := (uint32(c) >> 8) & 0xff
	b := (uint32(c) >> 16) & 0xff
	return uintptr(0xff000000 | r<<16 | g<<8 | b)
}

// fillRoundRectAA はアンチエイリアス付きで角丸四角形を塗る。成功したらtrue。
func fillRoundRectAA(hdc win.HDC, r win.RECT, radius int32, col win.COLORREF) bool {
	if !gdiplusReady {
		return false
	}
	var g, brush, path uintptr
	if s, _, _ := pGdipCreateFromHDC.Call(uintptr(hdc), uintptr(unsafe.Pointer(&g))); s != 0 {
		return false
	}
	defer pGdipDeleteGraphics.Call(g)
	pGdipSetSmoothing.Call(g, 4) // SmoothingModeAntiAlias
	if s, _, _ := pGdipCreateSolidFill.Call(argb(col), uintptr(unsafe.Pointer(&brush))); s != 0 {
		return false
	}
	defer pGdipDeleteBrush.Call(brush)
	if s, _, _ := pGdipCreatePath.Call(0, uintptr(unsafe.Pointer(&path))); s != 0 {
		return false
	}
	defer pGdipDeletePath.Call(path)

	w, h := r.Right-r.Left, r.Bottom-r.Top
	if radius*2 > h {
		radius = h / 2
	}
	if radius*2 > w {
		radius = w / 2
	}
	d := float32(radius * 2)
	l, t := float32(r.Left), float32(r.Top)
	rr, b := float32(r.Right), float32(r.Bottom)
	if radius <= 0 {
		d = 0.01
	}
	pGdipAddPathArc.Call(path, f32(l), f32(t), f32(d), f32(d), f32(180), f32(90))
	pGdipAddPathArc.Call(path, f32(rr-d), f32(t), f32(d), f32(d), f32(270), f32(90))
	pGdipAddPathArc.Call(path, f32(rr-d), f32(b-d), f32(d), f32(d), f32(0), f32(90))
	pGdipAddPathArc.Call(path, f32(l), f32(b-d), f32(d), f32(d), f32(90), f32(90))
	pGdipClosePathFigure.Call(path)
	pGdipFillPath.Call(g, brush, path)
	return true
}
