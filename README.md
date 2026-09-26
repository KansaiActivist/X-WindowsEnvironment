# 
## ビルド方法
前提としてcgoを使うため `CGO_ENABLED=1` とCコンパイラが必要です。
### MacOS
```bash
go mod tidy
go build -o x-browser .
./x-browser
```
### Windows
WebView2 SDK/ランタイムが必要です(Windows 11は標準搭載)。
MinGW-w64(TDM-GCCなど)でcgoをビルドできる環境を用意してください。
```powershell
go mod tidy
go build -ldflags="-H windowsgui" -o x-browser.exe .
```
`-H windowsgui` を付けるとバックグラウンドのコンソールウィンドウが表示されません。
### Linux
WebKitGTKの開発パッケージが必要です。
```bash
# Debian/Ubuntu の例
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev

go mod tidy
go build -o x-browser .
./x-browser
```
ディストリによっては `libwebkit2gtk-4.0-dev` の場合もあります。
## その他
Linuxの場合 `libwebkit2gtk` への依存
本アプリはX社のログイン処理やクッキーをOSのWebViewにそのまま委ねています。ログイン情報の保存場所はOSのWebViewエンジンの仕様に準拠します。




