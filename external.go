//go:build windows

package main

import "os/exec"

// openExternal はOSに設定されている既定のブラウザーでURLを開く。
// (Windowsの url.dll のFileProtocolHandlerを呼び出す定番の方法)
func openExternal(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
