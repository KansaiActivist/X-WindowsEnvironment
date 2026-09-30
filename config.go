//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lxn/win"
)

// Theme はタブバーの配色・丸み。config.json の "theme" で変更できる。
// 色は "#RRGGBB" 形式。空文字("")のものは preset("dark" または "light")の色が使われる。
type Theme struct {
	Preset          string `json:"preset"`
	BarBackground   string `json:"barBackground"`   // タブバー全体の背景
	TabActive       string `json:"tabActive"`       // 選択中のタブの背景
	TabInactive     string `json:"tabInactive"`     // 選択されていないタブの背景
	TabHover        string `json:"tabHover"`        // マウスを乗せたタブの背景
	TabText         string `json:"tabText"`         // タブの文字色
	TabTextInactive string `json:"tabTextInactive"` // 選択されていないタブの文字色
	Icon            string `json:"icon"`            // ボタンのアイコン色
	ButtonHover     string `json:"buttonHover"`     // ボタン/閉じる(タブ)のホバー背景
	CloseHover      string `json:"closeHover"`      // ウィンドウ右上「閉じる」のホバー背景
	CloseHoverIcon  string `json:"closeHoverIcon"`  // 同、アイコン色
	TabRadius       int    `json:"tabRadius"`       // タブ・ボタンの角の丸み(px)
}

type Config struct {
	Comment       string   `json:"_comment"`
	Zoom          float64  `json:"zoom"`
	Theme         Theme    `json:"theme"`
	Notifications string   `json:"notifications"` // "on" または "off"
	Profiles      []string `json:"profiles"`      // "Default"以外に追加したプロファイル名
}

var cfg = Config{Zoom: 1.0}

// palette は実際に描画に使う色(config.json から解決済み)。
var palette struct {
	bar, tabActive, tabInactive, tabHover   win.COLORREF
	tabText, tabTextInactive, icon          win.COLORREF
	buttonHover, closeHover, closeHoverIcon win.COLORREF
	radius                                  int32
}

var presets = map[string]Theme{
	"dark": {
		BarBackground: "#0F1419", TabActive: "#2F3336", TabInactive: "#0F1419", TabHover: "#1D2226",
		TabText: "#E7E9EA", TabTextInactive: "#8B98A5", Icon: "#C7CDD1",
		ButtonHover: "#2A3036", CloseHover: "#E81123", CloseHoverIcon: "#FFFFFF", TabRadius: 12,
	},
	"light": {
		BarBackground: "#DEE1E6", TabActive: "#FFFFFF", TabInactive: "#DEE1E6", TabHover: "#EEF0F3",
		TabText: "#202124", TabTextInactive: "#5F6368", Icon: "#3C4043",
		ButtonHover: "#CDD1D6", CloseHover: "#E81123", CloseHoverIcon: "#FFFFFF", TabRadius: 12,
	},
}

func appBaseDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "XBrowser")
}

func configPath() string { return filepath.Join(appBaseDir(), "config.json") }

// loadConfig は config.json を読み込む(無ければ既定値で作成する)。
func loadConfig() {
	_ = os.MkdirAll(appBaseDir(), 0o755)
	cfg = Config{
		Comment:       "色は #RRGGBB 形式。空文字は preset(dark/light) の色を使う。tabRadius は角の丸み(px)。notifications は on/off。変更後はアプリを再起動(通知は右クリメニューからも切替可)。",
		Zoom:          1.0,
		Theme:         Theme{Preset: "dark"},
		Notifications: "on",
	}
	if b, err := os.ReadFile(configPath()); err == nil {
		var c Config
		if json.Unmarshal(b, &c) == nil {
			if c.Zoom >= 0.25 && c.Zoom <= 5 {
				cfg.Zoom = c.Zoom
			}
			cfg.Theme = c.Theme
			if strings.ToLower(strings.TrimSpace(c.Notifications)) == "off" {
				cfg.Notifications = "off"
			}
			for _, p := range c.Profiles {
				p = strings.TrimSpace(p)
				if p != "" && !strings.EqualFold(p, "Default") {
					cfg.Profiles = append(cfg.Profiles, p)
				}
			}
		}
	}
	saveConfig() // 無いキーを補って書き戻し、ユーザーが編集しやすい形にする
	resolveTheme()
}

func saveConfig() {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(configPath(), b, 0o644)
	}
}

func pick(user, preset string) string {
	if strings.TrimSpace(user) != "" {
		return user
	}
	return preset
}

func parseHex(s string, fallback win.COLORREF) win.COLORREF {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return fallback
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return fallback
	}
	return rgb(byte(v>>16), byte(v>>8), byte(v))
}

func resolveTheme() {
	t := cfg.Theme
	base, ok := presets[strings.ToLower(t.Preset)]
	if !ok {
		base = presets["dark"]
	}
	palette.bar = parseHex(pick(t.BarBackground, base.BarBackground), parseHex(base.BarBackground, 0))
	palette.tabActive = parseHex(pick(t.TabActive, base.TabActive), parseHex(base.TabActive, 0))
	palette.tabInactive = parseHex(pick(t.TabInactive, base.TabInactive), parseHex(base.TabInactive, 0))
	palette.tabHover = parseHex(pick(t.TabHover, base.TabHover), parseHex(base.TabHover, 0))
	palette.tabText = parseHex(pick(t.TabText, base.TabText), parseHex(base.TabText, 0))
	palette.tabTextInactive = parseHex(pick(t.TabTextInactive, base.TabTextInactive), parseHex(base.TabTextInactive, 0))
	palette.icon = parseHex(pick(t.Icon, base.Icon), parseHex(base.Icon, 0))
	palette.buttonHover = parseHex(pick(t.ButtonHover, base.ButtonHover), parseHex(base.ButtonHover, 0))
	palette.closeHover = parseHex(pick(t.CloseHover, base.CloseHover), parseHex(base.CloseHover, 0))
	palette.closeHoverIcon = parseHex(pick(t.CloseHoverIcon, base.CloseHoverIcon), parseHex(base.CloseHoverIcon, 0))
	palette.radius = int32(base.TabRadius)
	if t.TabRadius > 0 {
		palette.radius = int32(t.TabRadius)
	}
}
