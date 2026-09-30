//go:build windows

package main

import (
	"crypto/sha1"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxn/win"
)

// Xのアカウント切替はWeb版だと5個までしか登録できないため、Cookieの入れ物
// (WebView2の「プロファイル」)自体を複数用意することで、実質的にいくつでも
// アカウントを管理できるようにする。各プロファイルは完全に独立した
// ログイン状態(Cookie)を持つ。

var slugPattern = regexp.MustCompile(`[^0-9A-Za-z一-龠ぁ-んァ-ヶー_-]+`)

func profileLabel(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Default"
	}
	return name
}

// profileDataDir はプロファイル名からWebView2のユーザーデータフォルダを決める。
// 空文字/"Default" は、これまでどおり単一プロファイル時代からのフォルダを使う
// (既存のログイン状態を引き継ぐため)。それ以外は名前ごとに別フォルダにする。
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

// allProfiles は "Default" を先頭に、config.json に保存された追加プロファイルを返す。
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

// addProfile は新しいプロファイル名を設定に追加して保存する。
func addProfile(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || profileExists(name) {
		return false
	}
	cfg.Profiles = append(cfg.Profiles, name)
	saveConfig()
	return true
}

// profileColor はプロファイル名からタブに表示する目印の色を決める
// (同じ名前なら常に同じ色になるように、名前のハッシュから選ぶ)。
var profileDotColors = []win.COLORREF{
	rgb(0x1D, 0x9B, 0xF0), // 青
	rgb(0x17, 0xBF, 0x63), // 緑
	rgb(0xF9, 0x1A, 0x80), // ピンク
	rgb(0xFF, 0xAD, 0x1F), // オレンジ
	rgb(0x79, 0x56, 0xF2), // 紫
	rgb(0x00, 0xC2, 0xB8), // ティール
	rgb(0xE0, 0x24, 0x5E), // 赤
	rgb(0xC4, 0xA0, 0x00), // 黄土
}

func profileColor(name string) win.COLORREF {
	if strings.TrimSpace(name) == "" || strings.EqualFold(name, "Default") {
		return 0 // Defaultは目印を付けない
	}
	sum := sha1.Sum([]byte(strings.ToLower(name)))
	return profileDotColors[int(sum[0])%len(profileDotColors)]
}
