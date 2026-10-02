//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var pAppendMenuW = user32b.NewProc("AppendMenuW")

func appendMenu(m win.HMENU, flags uint32, id uintptr, text string) {
	var p *uint16
	if flags&win.MF_SEPARATOR == 0 {
		p, _ = syscall.UTF16PtrFromString(text)
	}
	pAppendMenuW.Call(uintptr(m), uintptr(flags), id, uintptr(unsafe.Pointer(p)))
}

const (
	idNewTab            = 1
	idCloseTab          = 2
	idZoomReset         = 3
	idThemeDark         = 10
	idThemeLight        = 11
	idResetColors       = 12
	idOpenConfig        = 13
	idNotifications     = 14
	idProfileAdd        = 15
	idColorBase         = 100
	idRadiusBase        = 200
	idProfileNewBase    = 300 
	idProfileSwitchBase = 400 
)

type colorItem struct {
	label string
	field *string
	cur   *win.COLORREF
}

func colorItems() []colorItem {
	t := &cfg.Theme
	return []colorItem{
		{"タブバーの背景", &t.BarBackground, &palette.bar},
		{"選択中のタブ", &t.TabActive, &palette.tabActive},
		{"選択されていないタブ", &t.TabInactive, &palette.tabInactive},
		{"マウスを乗せたタブ", &t.TabHover, &palette.tabHover},
		{"タブの文字", &t.TabText, &palette.tabText},
		{"選択されていないタブの文字", &t.TabTextInactive, &palette.tabTextInactive},
		{"アイコン", &t.Icon, &palette.icon},
		{"ボタン/タブ×のホバー背景", &t.ButtonHover, &palette.buttonHover},
		{"右上「閉じる」のホバー背景", &t.CloseHover, &palette.closeHover},
		{"右上「閉じる」のホバー時アイコン", &t.CloseHoverIcon, &palette.closeHoverIcon},
	}
}

var radiusChoices = []struct {
	v     int
	label string
}{{2, "2 px(ほぼ角ばる)"}, {6, "6 px"}, {10, "10 px"}, {12, "12 px(標準)"}, {16, "16 px"}, {20, "20 px(丸い)"}}

func colorHex(c win.COLORREF) string {
	return fmt.Sprintf("#%02X%02X%02X", uint32(c)&0xff, (uint32(c)>>8)&0xff, (uint32(c)>>16)&0xff)
}

func pickColor(owner win.HWND, initial win.COLORREF) (win.COLORREF, bool) {
	var custom [16]win.COLORREF
	cc := win.CHOOSECOLOR{
		LStructSize:  uint32(unsafe.Sizeof(win.CHOOSECOLOR{})),
		HwndOwner:    owner,
		RgbResult:    initial,
		LpCustColors: &custom,
		Flags:        win.CC_RGBINIT | win.CC_FULLOPEN,
	}
	if !win.ChooseColor(&cc) {
		return 0, false
	}
	return cc.RgbResult, true
}

func themeChanged() {
	resolveTheme()
	saveConfig()
	invalidateBar()
}

func showContextMenu(hwnd win.HWND, x, y int32) {
	zone, tabIdx := app.hitTest(x, y)

	menu := win.CreatePopupMenu()
	defer win.DestroyMenu(menu) 

	appendMenu(menu, win.MF_STRING, idNewTab, "新しいタブ")

	profiles := allProfiles()
	newTabProfiles := win.CreatePopupMenu()
	for i, p := range profiles {
		label := p
		if p == "Default" {
			label = "Default(通常)"
		}
		appendMenu(newTabProfiles, win.MF_STRING, uintptr(idProfileNewBase+i), label)
	}
	appendMenu(newTabProfiles, win.MF_SEPARATOR, 0, "")
	appendMenu(newTabProfiles, win.MF_STRING, idProfileAdd, "新しいプロファイルを追加…")
	appendMenu(menu, win.MF_POPUP, uintptr(newTabProfiles), "プロファイルを選んで新しいタブ")

	if zone == zoneTab || zone == zoneTabClose {
		appendMenu(menu, win.MF_STRING, idCloseTab, "このタブを閉じる")

		switchMenu := win.CreatePopupMenu()
		curProfile := profileLabel(app.tabs.tabs[tabIdx].profile)
		for i, p := range profiles {
			label := p
			if p == "Default" {
				label = "Default(通常)"
			}
			f := uint32(win.MF_STRING)
			if strings.EqualFold(p, curProfile) {
				f |= win.MF_CHECKED
			}
			appendMenu(switchMenu, f, uintptr(idProfileSwitchBase+i), label)
		}
		appendMenu(menu, win.MF_POPUP, uintptr(switchMenu), fmt.Sprintf("このタブのプロファイル(現在: %s)", curProfile))
	}
	appendMenu(menu, win.MF_SEPARATOR, 0, "")

	settings := win.CreatePopupMenu()
	preset := strings.ToLower(cfg.Theme.Preset)
	chk := func(b bool) uint32 {
		if b {
			return win.MF_STRING | win.MF_CHECKED
		}
		return win.MF_STRING
	}
	appendMenu(settings, chk(preset != "light"), idThemeDark, "テーマ: ダーク")
	appendMenu(settings, chk(preset == "light"), idThemeLight, "テーマ: ライト")
	appendMenu(settings, win.MF_SEPARATOR, 0, "")

	colors := win.CreatePopupMenu()
	for i, it := range colorItems() {
		appendMenu(colors, win.MF_STRING, uintptr(idColorBase+i), fmt.Sprintf("%s  (%s)…", it.label, colorHex(*it.cur)))
	}
	appendMenu(settings, win.MF_POPUP, uintptr(colors), "色を編集")

	radius := win.CreatePopupMenu()
	for i, rc := range radiusChoices {
		appendMenu(radius, chk(int32(rc.v) == palette.radius), uintptr(idRadiusBase+i), rc.label)
	}
	appendMenu(settings, win.MF_POPUP, uintptr(radius), "角の丸み")

	appendMenu(settings, win.MF_SEPARATOR, 0, "")
	appendMenu(settings, chk(cfg.Notifications != "off"), idNotifications, "Xのプッシュ通知")
	appendMenu(settings, win.MF_SEPARATOR, 0, "")
	appendMenu(settings, win.MF_STRING, idResetColors, "色を初期値に戻す")
	appendMenu(settings, win.MF_STRING, idOpenConfig, "設定ファイル(config.json)を開く")
	appendMenu(menu, win.MF_POPUP, uintptr(settings), "設定")

	appendMenu(menu, win.MF_STRING, idZoomReset, "拡大率を100%に戻す")

	pt := win.POINT{X: x, Y: y}
	win.ClientToScreen(hwnd, &pt)
	win.SetForegroundWindow(hwnd)
	cmd := win.TrackPopupMenu(menu, win.TPM_RETURNCMD|win.TPM_RIGHTBUTTON, pt.X, pt.Y, 0, hwnd, nil)
	win.PostMessage(hwnd, win.WM_NULL, 0, 0)

	app.hovered, app.hoverTab = zoneNone, -1
	invalidateBar()

	switch {
	case cmd == idNewTab:
		app.tabs.AddTab(homeURL, app.tabs.Active().profile)
		app.layout()
	case cmd == idCloseTab:
		app.tabs.Close(tabIdx)
		app.layout()
	case cmd == idProfileAdd:
		if name, ok := showInputBox(hwnd, "新しいプロファイル", "プロファイル名(例: 仕事用、サブ垢1 など):", ""); ok {
			name = strings.TrimSpace(name)
			if name != "" && !strings.EqualFold(name, "Default") {
				addProfile(name)
				app.tabs.AddTab(homeURL, name)
				app.layout()
			}
		}
	case cmd >= idProfileNewBase && cmd < idProfileNewBase+100:
		if i := int(cmd - idProfileNewBase); i < len(profiles) {
			p := profiles[i]
			if p == "Default" {
				p = ""
			}
			app.tabs.AddTab(homeURL, p)
			app.layout()
		}
	case cmd >= idProfileSwitchBase && cmd < idProfileSwitchBase+100:
		if i := int(cmd - idProfileSwitchBase); i < len(profiles) && tabIdx >= 0 {
			p := profiles[i]
			if p == "Default" {
				p = ""
			}
			app.tabs.SwitchTabProfile(tabIdx, p)
			app.layout()
		}
	case cmd == idZoomReset:
		app.tabs.ZoomReset()
	case cmd == idThemeDark, cmd == idThemeLight:
		p := "dark"
		if cmd == idThemeLight {
			p = "light"
		}
		cfg.Theme = Theme{Preset: p, TabRadius: cfg.Theme.TabRadius}
		themeChanged()
	case cmd == idResetColors:
		cfg.Theme = Theme{Preset: cfg.Theme.Preset}
		themeChanged()
	case cmd == idOpenConfig:
		_ = exec.Command("notepad.exe", configPath()).Start()
	case cmd == idNotifications:
		if cfg.Notifications == "off" {
			cfg.Notifications = "on"
		} else {
			cfg.Notifications = "off"
		}
		saveConfig()
		app.tabs.applyNotificationSettingAll()
	case cmd >= idColorBase && cmd < idColorBase+100:
		items := colorItems()
		if i := int(cmd - idColorBase); i < len(items) {
			if c, ok := pickColor(hwnd, *items[i].cur); ok {
				*items[i].field = colorHex(c)
				themeChanged()
			}
		}
	case cmd >= idRadiusBase && cmd < idRadiusBase+100:
		if i := int(cmd - idRadiusBase); i < len(radiusChoices) {
			cfg.Theme.TabRadius = radiusChoices[i].v
			themeChanged()
		}
	}
}
