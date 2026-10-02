//go:build windows

package main

import "os/exec"


func openExternal(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
