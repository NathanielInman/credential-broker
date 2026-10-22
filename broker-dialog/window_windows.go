package main

import (
	"os"
	"path/filepath"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	setWindowPos     = user32.NewProc("SetWindowPos")
	setForegroundWnd = user32.NewProc("SetForegroundWindow")
)

const (
	hwndTopmost  = ^uintptr(0) // HWND_TOPMOST, (HWND)-1
	swpNoSize    = 0x0001
	swpNoMove    = 0x0002
	swpShowWnd   = 0x0040
	swpTopmostFl = swpNoSize | swpNoMove | swpShowWnd
)

// wv2Window is a WebView2 window. Closing it by its X ends Run the same way
// quit does, so quitting tracks that it asked.
type wv2Window struct {
	w        webview2.WebView
	width    int
	quitting bool
}

func newWindow(html string, width, height int) window {
	data := filepath.Join(os.Getenv("LOCALAPPDATA"), "broker", "webview2")
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  data,
		WindowOptions: webview2.WindowOptions{
			Title:  "broker request",
			Width:  uint(width),
			Height: uint(height),
			Center: true,
		},
	})
	if w == nil {
		os.Exit(1)
	}
	win := &wv2Window{w: w, width: width}
	// The page posts through this binding where WebKit has
	// window.webkit.messageHandlers.
	_ = w.Bind("brokerPost", func(line string) { onMessage(line) })
	w.SetSize(width, height, webview2.HintFixed)
	hwnd := uintptr(w.Window())
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpTopmostFl)
	setForegroundWnd.Call(hwnd)
	w.SetHtml(html)
	return win
}

func (v *wv2Window) run() {
	v.w.Run()
	// Run ended without a quit: the human closed the window.
	if !v.quitting {
		onClosed()
	}
	v.w.Destroy()
}

func (v *wv2Window) resize(h int) {
	if h < 120 {
		h = 120
	}
	if h > 900 {
		h = 900
	}
	v.w.SetSize(v.width, h, webview2.HintFixed)
}

func (v *wv2Window) eval(js string) {
	v.w.Dispatch(func() { v.w.Eval(js) })
}

func (v *wv2Window) quit() {
	v.w.Dispatch(func() {
		v.quitting = true
		v.w.Terminate()
	})
}
