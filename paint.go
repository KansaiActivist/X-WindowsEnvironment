//go:build windows

package main

import (
	"fmt"
	"syscall"

	"github.com/lxn/win"
)

func rgb(r, g, b byte) win.COLORREF {
	return win.COLORREF(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

const iconFace = "Segoe MDL2 Assets"

func makeFont(face string, height int32, weight int32) win.HFONT {
	var lf win.LOGFONT
	lf.LfHeight = height
	lf.LfWeight = weight
	lf.LfCharSet = 1 
	lf.LfQuality = 5 
	u := syscall.StringToUTF16(face)
	for i := 0; i < len(u) && i < len(lf.LfFaceName)-1; i++ {
		lf.LfFaceName[i] = u[i]
	}
	return win.CreateFontIndirect(&lf)
}

type painter struct {
	hdc                                    win.HDC
	fText, fTextBold, fIcon, fIconS, fZoom win.HFONT
}

func (p *painter) fill(r win.RECT, col win.COLORREF) {
	brush := createSolidBrush(col)
	winFillRect(p.hdc, &r, brush)
	win.DeleteObject(win.HGDIOBJ(brush))
}

func (p *painter) round(r win.RECT, radius int32, col win.COLORREF) {
	if fillRoundRectAA(p.hdc, r, radius, col) {
		return
	}
	brush := createSolidBrush(col)
	oldB := win.SelectObject(p.hdc, win.HGDIOBJ(brush))
	oldP := win.SelectObject(p.hdc, win.GetStockObject(win.NULL_PEN))
	win.RoundRect(p.hdc, r.Left, r.Top, r.Right+1, r.Bottom+1, radius*2, radius*2)
	win.SelectObject(p.hdc, oldP)
	win.SelectObject(p.hdc, oldB)
	win.DeleteObject(win.HGDIOBJ(brush))
}

func (p *painter) text(r win.RECT, s string, col win.COLORREF, font win.HFONT, flags uint32) {
	old := win.SelectObject(p.hdc, win.HGDIOBJ(font))
	win.SetTextColor(p.hdc, col)
	rc := r
	u, n := utf16(s)
	winDrawText(p.hdc, u, n, &rc, win.DT_SINGLELINE|win.DT_VCENTER|flags)
	win.SelectObject(p.hdc, old)
}

func (p *painter) icon(r win.RECT, glyph string, col win.COLORREF, small bool) {
	f := p.fIcon
	if small {
		f = p.fIconS
	}
	p.text(r, glyph, col, f, win.DT_CENTER)
}

func utf16(s string) (*uint16, int32) {
	u := syscall.StringToUTF16(s)
	return &u[0], int32(len(u) - 1)
}

func inset(r win.RECT, dx, dy int32) win.RECT {
	return win.RECT{Left: r.Left + dx, Top: r.Top + dy, Right: r.Right - dx, Bottom: r.Bottom - dy}
}

func paintChrome(hwnd win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)

	var client win.RECT
	win.GetClientRect(hwnd, &client)
	w, h := client.Right-client.Left, int32(titleBarH)

	mem := win.CreateCompatibleDC(hdc)
	bmp := win.CreateCompatibleBitmap(hdc, w, h)
	oldBmp := win.SelectObject(mem, win.HGDIOBJ(bmp))
	win.SetBkMode(mem, win.TRANSPARENT)

	p := &painter{
		hdc:       mem,
		fText:     makeFont("Segoe UI", -15, win.FW_NORMAL),
		fTextBold: makeFont("Segoe UI", -15, 600),
		fIcon:     makeFont(iconFace, -14, win.FW_NORMAL),
		fIconS:    makeFont(iconFace, -10, win.FW_NORMAL),
		fZoom:     makeFont("Segoe UI", -13, 600),
	}

	p.fill(win.RECT{Left: 0, Top: 0, Right: w, Bottom: h}, palette.bar)
	p.drawNav()
	p.drawTabs()
	p.drawNewTab()
	p.drawZoom()
	p.drawWindowButtons()

	win.BitBlt(hdc, 0, 0, w, h, mem, 0, 0, win.SRCCOPY)

	for _, f := range []win.HFONT{p.fText, p.fTextBold, p.fIcon, p.fIconS, p.fZoom} {
		win.DeleteObject(win.HGDIOBJ(f))
	}
	win.SelectObject(mem, oldBmp)
	win.DeleteObject(win.HGDIOBJ(bmp))
	win.DeleteDC(mem)
}

func (p *painter) hoverPill(zone hitZone, r win.RECT) {
	if app.hovered == zone {
		p.round(r, palette.radius, palette.buttonHover)
	}
}

func (p *painter) drawNav() {
	items := []struct {
		z hitZone
		g string
	}{{zoneBack, "\uE72B"}, {zoneForward, "\uE72A"}, {zoneReload, "\uE72C"}}
	for _, it := range items {
		slot := app.navButtonRect(it.z)
		pill := inset(slot, 3, 8)
		p.hoverPill(it.z, pill)
		p.icon(slot, it.g, palette.icon, false)
	}
}

func (p *painter) drawNewTab() {
	slot := app.newTabBtnRect()
	p.hoverPill(zoneNewTab, inset(slot, 3, 8))
	p.icon(slot, "\uE710", palette.icon, false)
}

func (p *painter) drawTabs() {
	for i, slot := range app.tabRects() {
		tab := app.tabs.tabs[i]
		vr := tabVisualRect(slot)
		active := i == app.tabs.active
		hovered := app.hoverTab == i && (app.hovered == zoneTab || app.hovered == zoneTabClose)

		bg, txt, font := palette.tabInactive, palette.tabTextInactive, p.fText
		if active {
			bg, txt, font = palette.tabActive, palette.tabText, p.fTextBold
		} else if hovered {
			bg = palette.tabHover
		}
		if bg != palette.bar {
			p.round(vr, palette.radius, bg)
		}

		title := tab.title
		if title == "" {
			title = "X"
		}
		cr := tabCloseRect(slot)
		textLeft := vr.Left + int32(14)
		if dot := profileColor(tab.profile); dot != 0 {
			const d = 8
			dotTop := vr.Top + (vr.Bottom-vr.Top-d)/2
			p.round(win.RECT{Left: vr.Left + 10, Top: dotTop, Right: vr.Left + 10 + d, Bottom: dotTop + d}, d/2, dot)
			textLeft = vr.Left + 26
		}
		tr := win.RECT{Left: textLeft, Top: vr.Top, Right: cr.Left - 4, Bottom: vr.Bottom}
		p.text(tr, title, txt, font, win.DT_LEFT|win.DT_END_ELLIPSIS)

		if app.hovered == zoneTabClose && app.hoverTab == i {
			p.round(cr, (cr.Bottom-cr.Top)/2, palette.buttonHover)
		}
		closeCol := palette.tabTextInactive
		if active || hovered {
			closeCol = palette.tabText
		}
		p.icon(cr, "\uE711", closeCol, true)
	}
}

func (p *painter) drawZoom() {
	out, lab, in := app.zoomRect(zoneZoomOut), app.zoomRect(zoneZoomLabel), app.zoomRect(zoneZoomIn)
	whole := win.RECT{Left: out.Left, Top: out.Top, Right: in.Right, Bottom: out.Bottom}
	p.round(whole, (whole.Bottom-whole.Top)/2, palette.tabHover)
	p.hoverPill(zoneZoomOut, inset(out, 2, 2))
	p.hoverPill(zoneZoomLabel, inset(lab, 0, 2))
	p.hoverPill(zoneZoomIn, inset(in, 2, 2))
	p.icon(out, "\uE71F", palette.icon, true)
	p.icon(in, "\uE8A3", palette.icon, true)
	label := fmt.Sprintf("%d%%", int(cfg.Zoom*100+0.5))
	p.text(lab, label, palette.tabText, p.fZoom, win.DT_CENTER)
}

func (p *painter) drawWindowButtons() {
	draw := func(zone hitZone, glyph string) {
		r := app.winButtonRect(zone)
		col := palette.icon
		if app.hovered == zone {
			if zone == zoneClose {
				p.fill(r, palette.closeHover)
				col = palette.closeHoverIcon
			} else {
				p.fill(r, palette.buttonHover)
			}
		}
		p.icon(r, glyph, col, true)
	}
	draw(zoneMinimize, "\uE921")
	max := "\uE922"
	if isMaximized(app.hwnd) {
		max = "\uE923"
	}
	draw(zoneMaximize, max)
	draw(zoneClose, "\uE8BB")
}
