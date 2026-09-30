//go:build windows

package main

import (
	"encoding/json"
	"os"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
	"github.com/lxn/win"
)

const tabContainerClass = "XBrowserTabContainer"

func registerTabContainerClass(hInstance win.HINSTANCE) {
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(win.DefWindowProc),
		HInstance:     hInstance,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		LpszClassName: syscall.StringToUTF16Ptr(tabContainerClass),
	}
	win.RegisterClassEx(&wc)
}

// Tab は1つのタブ = 1つの子HWND + 1つのWebView2コントローラ。
// タブごとに別のWebView2コントローラを持つが、DataPathを共有しているので
// 裏側のEdgeブラウザプロセス/ログイン状態は1つにまとまり、メモリを節約できる。
type Tab struct {
	hwnd     win.HWND
	chromium *edge.Chromium
	title    string
	url      string
	profile  string // このタブが使うプロファイル名("" はDefault扱い)
	ready    bool   // Embed完了後にtrue
}

func (t *Tab) Eval(js string) {
	if t.chromium != nil {
		t.chromium.Eval(js)
	}
}

type TabManager struct {
	parent      win.HWND
	tabs        []*Tab
	active      int
	lastContent win.RECT
}

func newTabManager(parent win.HWND) *TabManager {
	initDebugLog(appBaseDir())
	debugLog("TabManager作成 baseDir=%s", appBaseDir())
	return &TabManager{parent: parent, active: -1}
}

func (m *TabManager) Active() *Tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return &Tab{}
	}
	return m.tabs[m.active]
}

// AddTab は指定したプロファイル("" ならDefault)で新しいタブを開く。
func (m *TabManager) AddTab(url string, profile string) {
	hInstance := win.GetModuleHandle(nil)
	child := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr(tabContainerClass),
		syscall.StringToUTF16Ptr(""),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_CLIPCHILDREN|win.WS_CLIPSIBLINGS,
		0, 0, 10, 10,
		m.parent, 0, hInstance, nil,
	)

	tab := &Tab{hwnd: child, url: url, title: "新しいタブ", profile: profile}
	m.tabs = append(m.tabs, tab)
	newIndex := len(m.tabs) - 1

	chromium := edge.NewChromium()
	chromium.DataPath = profileDataDir(profile)
	debugLog("プロファイル=%s dataDir=%s", profileLabel(profile), chromium.DataPath)
	chromium.MessageCallback = func(msg string) { handleTabMessage(tab, msg) }
	chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
		debugLog("NavigationCompleted url=%s", url)
		applyZoomTo(chromium)
	}
	tab.chromium = chromium
	chromium.AcceleratorKeyCallback = func(vk uint) bool {
		if win.GetKeyState(win.VK_CONTROL) >= 0 {
			return false
		}
		switch vk {
		case 0xBB, 0x6B: // + (=) / テンキー+
			m.ZoomStep(1)
			return true
		case 0xBD, 0x6D: // - / テンキー-
			m.ZoomStep(-1)
			return true
		case 0x30, 0x60: // 0 / テンキー0
			m.ZoomReset()
			return true
		}
		return false
	}

	debugLog("Embed開始 url=%s", url)
	if !chromium.Embed(uintptr(child)) {
		debugLog("Embed失敗")
		// WebView2ランタイムが見つからない等。ユーザーに知らせて終了。
		win.MessageBox(m.parent,
			mustUTF16("WebView2 ランタイムの初期化に失敗しました。\r\nMicrosoft Edge WebView2 Runtime をインストールしてください。"),
			mustUTF16("xbrowser"), win.MB_ICONERROR)
		os.Exit(1)
	}
	debugLog("Embed成功")
	tab.ready = true
	applyZoomTo(chromium)

	if settings, err := chromium.GetSettings(); err == nil {
		_ = settings.PutAreDefaultContextMenusEnabled(true)
		_ = settings.PutAreDevToolsEnabled(false)
		// 拡大縮小は自前で処理する(Ctrl+ホイール / Ctrl+ +,-,0 とバー上のボタン)
		_ = settings.PutIsZoomControlEnabled(false)
	}

	// WebView2は読み込み前後に既定で背景が黒になることがあり、
	// 真っ黒な画面のまま何も表示されないように見える原因になる。
	// 明示的に白へ変更しておく。
	if controller := chromium.GetController(); controller != nil {
		if c2 := controller.GetICoreWebView2Controller2(); c2 != nil {
			_ = c2.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{A: 255, R: 255, G: 255, B: 255})
		}
	}

	applyNotificationSetting(chromium)

	chromium.Init(injectedJS)
	debugLog("Navigate呼び出し url=%s", url)
	chromium.Navigate(url)

	m.Switch(newIndex)
}

// SwitchTabProfile は既存タブを閉じて、同じURLを別プロファイルで開き直す
// (WebView2は作成後にプロファイル/DataPathを変更できないため)。
// タブの並び順(位置)は維持する。
func (m *TabManager) SwitchTabProfile(idx int, profile string) {
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	url := m.tabs[idx].url
	if url == "" {
		url = homeURL
	}
	win.DestroyWindow(m.tabs[idx].hwnd)
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)

	m.AddTab(url, profile) // 末尾に追加され、アクティブになる
	last := len(m.tabs) - 1
	t := m.tabs[last]
	m.tabs = m.tabs[:last]
	tail := append([]*Tab{t}, m.tabs[idx:]...)
	m.tabs = append(m.tabs[:idx], tail...)
	m.active = idx
}

func (m *TabManager) Switch(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	for idx, t := range m.tabs {
		if idx != i {
			win.ShowWindow(t.hwnd, win.SW_HIDE)
			if t.ready {
				_ = t.chromium.Hide()
			}
		}
	}
	m.active = i
	win.ShowWindow(m.tabs[i].hwnd, win.SW_SHOW)
	m.Refresh()
	if m.tabs[i].ready {
		m.tabs[i].chromium.Focus()
	}
}

// Refresh はアクティブなタブのWebView2を「表示状態」にして、サイズと位置を再通知する。
// WebView2は非表示の親ウィンドウの下で作られると、可視状態が更新されず
// 描画されない(真っ黒のまま)ことがあるため、明示的に表示させる。
func (m *TabManager) Refresh() {
	if m.active < 0 || m.active >= len(m.tabs) {
		return
	}
	t := m.tabs[m.active]
	if !t.ready {
		return
	}
	m.resizeActive()
	{
		_ = t.chromium.Show()
		_ = t.chromium.NotifyParentWindowPositionChanged()
	}
}

func (m *TabManager) Close(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	win.DestroyWindow(m.tabs[i].hwnd)
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)

	if len(m.tabs) == 0 {
		win.DestroyWindow(m.parent)
		return
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	m.Switch(m.active)
}

func (m *TabManager) Resize(content win.RECT) {
	m.lastContent = content
	m.resizeActive()
}

func (m *TabManager) resizeActive() {
	if m.active < 0 || m.active >= len(m.tabs) {
		return
	}
	r := m.lastContent
	win.MoveWindow(m.tabs[m.active].hwnd, r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top, true)
	if m.tabs[m.active].ready {
		m.tabs[m.active].chromium.Resize()
	}
}

type tabMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func handleTabMessage(tab *Tab, raw string) {
	debugLog("MessageCallback raw=%s", raw)
	var msg tabMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return
	}
	switch msg.Type {
	case "open_external":
		openExternal(msg.Data)
	case "title":
		tab.title = msg.Data
		win.InvalidateRect(app.hwnd, nil, false)
	case "zoom":
		switch msg.Data {
		case "in":
			app.tabs.ZoomStep(1)
		case "out":
			app.tabs.ZoomStep(-1)
		case "reset":
			app.tabs.ZoomReset()
		}
	case "url":
		tab.url = msg.Data
	}
}
