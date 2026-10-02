//go:build windows

package main

import (
	"crypto/sha1"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxn/win"
)


var slugPattern = regexp.MustCompile(`[^0-9A-Za-z一-龠ぁ-んァ-ヶー_-]+`)

func profileLabel(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Default"
	}
	return name
}


func profileDataDir(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "Default") {
		return filepath.Join(appBaseDir(), "WebView2Data")
	}
	slug := slugPattern.ReplaceAllString(name, "_")
	if slug == "" {
		slug = "profile"
	}
	return filepath.Join(appBaseDir(), "WebView2Data-"+slug)
}

func allProfiles() []string {
	return append([]string{"Default"}, cfg.Profiles...)
}

func profileExists(name string) bool {
	name = strings.TrimSpace(name)
	for _, p := range allProfiles() {
		if strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

func addProfile(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || profileExists(name) {
		return false
	}
	cfg.Profiles = append(cfg.Profiles, name)
	saveConfig()
	return true
}

var profileDotColors = []win.COLORREF{
	rgb(0x1D, 0x9B, 0xF0), 
	rgb(0x17, 0xBF, 0x63), 
	rgb(0xF9, 0x1A, 0x80), 
	rgb(0xFF, 0xAD, 0x1F), 
	rgb(0x79, 0x56, 0xF2), 
	rgb(0x00, 0xC2, 0xB8), 
	rgb(0xE0, 0x24, 0x5E), 
	rgb(0xC4, 0xA0, 0x00), 
}

func profileColor(name string) win.COLORREF {
	if strings.TrimSpace(name) == "" || strings.EqualFold(name, "Default") {
		return 0 
	}
	sum := sha1.Sum([]byte(strings.ToLower(name)))
	return profileDotColors[int(sum[0])%len(profileDotColors)]
}
