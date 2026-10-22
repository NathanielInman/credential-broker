// Command broker-dialog is broker's request window, the desktop dialog in
// which a human approves an agent's request for secrets.
//
// broker runs it and speaks JSON lines with it: messages to show arrive on
// stdin and go to the page as they are; what the human does comes back on
// stdout. The page (ui.html) holds all the presentation and none of the
// decisions, and nothing it shows is ever parsed as HTML.
//
// It is a separate program so broker itself stays pure Go, with no GUI
// toolkit to link. The window is GTK and WebKitGTK on linux
// (window_linux.c), Cocoa and WKWebView on macOS (window_darwin.m), and
// WebView2 on Windows (window_windows.go); all show the same page.
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"runtime"
	"sync"
)

//go:embed ui.html
var page string

const (
	width  = 470
	height = 560
	// maxMessage bounds one line from broker.
	maxMessage = 1 << 20
)

// window is the platform's request window. run blocks on the GUI thread
// until quit or the human closes the window; the other methods are safe
// from any goroutine except resize, which is called on the GUI thread.
type window interface {
	run()
	resize(height int)
	eval(js string)
	quit()
}

var (
	// proto is where replies go: the real stdout, kept apart from fd 1 so
	// nothing a GUI library prints can corrupt the protocol.
	proto *os.File
	send  sync.Mutex
	// replied is set once the window has said deny, so closing it after
	// does not say it again. Only the GUI thread touches it.
	replied bool

	ready     = make(chan struct{})
	readyOnce sync.Once

	win window
)

func init() {
	// GUI toolkits must run on the main thread.
	runtime.LockOSThread()
}

func main() {
	var err error
	if proto, err = protocolOut(); err != nil {
		os.Exit(1)
	}
	platformInit()
	win = newWindow(page, width, height)
	go pump()
	win.run()
	// The window is gone. If the human closed it, the deny is already out.
	send.Lock()
	proto.Close()
	send.Unlock()
}

// pump hands each message from broker to the page once it has loaded, and
// closes the window when broker goes away.
func pump() {
	<-ready
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 4096), maxMessage)
	for sc.Scan() {
		line := sc.Bytes()
		if !json.Valid(line) {
			continue
		}
		// The line goes in as a JS string literal for the page to parse,
		// never as code. json.Marshal escapes everything that matters.
		lit, _ := json.Marshal(string(line))
		win.eval("broker.receive(" + string(lit) + ")")
	}
	win.quit()
}

// reply forwards one line from the page to broker.
func reply(line string) {
	send.Lock()
	defer send.Unlock()
	_, _ = proto.WriteString(line + "\n")
}

// onMessage handles one message from the page, on the GUI thread.
func onMessage(line string) {
	var m struct {
		Type   string `json:"type"`
		Height int    `json:"height"`
	}
	if json.Unmarshal([]byte(line), &m) != nil {
		return
	}
	switch m.Type {
	case "ready":
		readyOnce.Do(func() { close(ready) })
	case "size":
		win.resize(m.Height)
	case "exit":
		win.quit()
	case "deny":
		replied = true
		reply(line)
	default:
		reply(line)
	}
}

// onClosed handles the human closing the window, which is a deny.
func onClosed() {
	if !replied {
		replied = true
		reply(`{"type":"deny"}`)
	}
}
