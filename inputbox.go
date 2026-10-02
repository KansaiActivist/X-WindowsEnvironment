//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const inputBoxClass = "XBrowserInputBox"

const (
	ibEdit   = win.HMENU(1)
	ibOK     = win.HMENU(2)
	ibCancel = win.HMENU(3)
)

type inputBoxState struct {
	edit   win.HWND
	result string
	ok     bool
	done   bool
}

var ibState *inputBoxState

func registerInputBoxClass(hInstance win.HINSTANCE) {
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(inputBoxWndProc),
		HInstance:     hInstance,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: win.HBRUSH(win.COLOR_BTNFACE + 1),
		LpszClassName: syscall.StringToUTF16Ptr(inputBoxClass),
	}
	win.RegisterClassEx(&wc)
}

func inputBoxWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_COMMAND:
		id := win.HMENU(wParam & 0xffff)
		if id == ibOK || id == ibCancel {
			if ibState != nil {
				ibState.ok = id == ibOK
				if ibState.ok {
					buf := make([]uint16, 256)
					n := getWindowText(ibState.edit, buf)
					ibState.result = syscall.UTF16ToString(buf[:n])
				}
				ibState.done = true
			}
			return 0
		}
	case win.WM_CLOSE:
		if ibState != nil {
			ibState.ok, ibState.done = false, true
		}
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func showInputBox(owner win.HWND, title, prompt, defaultText string) (string, bool) {
	hInstance := win.GetModuleHandle(nil)
	registerInputBoxClass(hInstance)

	const w, h = 360, 150
	x, y, _, _ := initialWindowRect() 
	var ownerRect win.RECT
	if win.GetWindowRect(owner, &ownerRect) {
		x = ownerRect.Left + (ownerRect.Right-ownerRect.Left-w)/2
		y = ownerRect.Top + (ownerRect.Bottom-ownerRect.Top-h)/2
	}

	style := uint32(win.WS_POPUP | win.WS_CAPTION | win.WS_SYSMENU)
	hwnd := win.CreateWindowEx(win.WS_EX_DLGMODALFRAME, syscall.StringToUTF16Ptr(inputBoxClass),
		syscall.StringToUTF16Ptr(title), style, x, y, w, h, owner, 0, hInstance, nil)
	if hwnd == 0 {
		return "", false
	}

	font := makeFont("Segoe UI", -14, win.FW_NORMAL)
	mk := func(class string, exStyle uint32, wstyle uint32, x, y, w, h int32, id win.HMENU, text string) win.HWND {
		c := win.CreateWindowEx(exStyle, syscall.StringToUTF16Ptr(class), syscall.StringToUTF16Ptr(text),
			wstyle|win.WS_CHILD|win.WS_VISIBLE, x, y, w, h, hwnd, id, hInstance, nil)
		win.SendMessage(c, win.WM_SETFONT, uintptr(font), 1)
		return c
	}
	mk("STATIC", 0, win.SS_LEFT, 16, 16, w-50, 20, 0, prompt)
	edit := mk("EDIT", win.WS_EX_CLIENTEDGE, win.WS_TABSTOP|win.ES_AUTOHSCROLL, 16, 42, w-50, 24, ibEdit, defaultText)
	mk("BUTTON", 0, win.WS_TABSTOP|win.BS_DEFPUSHBUTTON, w-170, 82, 80, 28, ibOK, "OK")
	mk("BUTTON", 0, win.WS_TABSTOP, w-82, 82, 66, 28, ibCancel, "キャンセル")

	setWindowText(edit, defaultText)
	win.SetFocus(edit)
	win.SendMessage(edit, win.EM_SETSEL, 0, ^uintptr(0))

	win.EnableWindow(owner, false)
	win.ShowWindow(hwnd, win.SW_SHOW)
	win.SetForegroundWindow(hwnd)

	ibState = &inputBoxState{edit: edit}
	for !ibState.done {
		var msg win.MSG
		r := win.GetMessage(&msg, 0, 0, 0)
		if r <= 0 {
			break
		}
		if msg.Message == win.WM_KEYDOWN && (msg.HWnd == hwnd || win.IsChild(hwnd, msg.HWnd)) {
			if msg.WParam == uintptr(win.VK_RETURN) {
				ibState.ok, ibState.done = true, true
				buf := make([]uint16, 256)
				n := getWindowText(edit, buf)
				ibState.result = syscall.UTF16ToString(buf[:n])
				break
			}
			if msg.WParam == uintptr(win.VK_ESCAPE) {
				ibState.ok, ibState.done = false, true
				break
			}
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}

	result, ok := ibState.result, ibState.ok
	ibState = nil
	win.EnableWindow(owner, true)
	win.SetForegroundWindow(owner)
	win.DestroyWindow(hwnd)
	win.DeleteObject(win.HGDIOBJ(font))
	return result, ok
}
