// Command broker-board is Signal Box's panel window: the view that springs
// out of broker's lamp in the bar.
//
// "broker board serve" runs it and speaks JSON lines with it, the same way
// broker speaks with broker-dialog: what to show arrives on stdin and goes
// to the page as it is; what the human does comes back on stdout. The page
// (ui.html) holds all the presentation and none of the decisions: it never
// sees a secret's value, and anything it asks for goes through broker.
//
// It is a separate program so broker itself stays pure Go. The window is a
// GTK 4 layer-shell surface with WebKitGTK 6 on linux (window_linux.c).
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

// maxMessage bounds one line from broker; a snapshot of a large log can be
// a few hundred kilobytes.
const maxMessage = 8 << 20

var (
	// proto is where replies go: the real stdout, kept apart from fd 1 so
	// nothing a GUI library prints can corrupt the protocol.
	proto *os.File
	send  sync.Mutex

	ready     = make(chan struct{})
	readyOnce sync.Once
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
	newWindow(page)
	go pump()
	runWindow()
	send.Lock()
	proto.Close()
	send.Unlock()
}

// pump hands each message from broker to the page once it has loaded,
// mapping the window first for show, and quits when broker goes away.
func pump() {
	<-ready
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64<<10), maxMessage)
	for sc.Scan() {
		line := sc.Bytes()
		var m struct {
			Type   string `json:"type"`
			Anchor *struct {
				Monitor string `json:"monitor"`
			} `json:"anchor"`
		}
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		if m.Type == "show" {
			monitor := ""
			if m.Anchor != nil {
				monitor = m.Anchor.Monitor
			}
			showWindow(monitor)
		}
		// The line goes in as a JS string literal for the page to parse,
		// never as code. json.Marshal escapes everything that matters.
		lit, _ := json.Marshal(string(line))
		evalPage("board.receive(" + string(lit) + ")")
	}
	quitWindow()
}

func reply(line string) {
	send.Lock()
	defer send.Unlock()
	_, _ = proto.WriteString(line + "\n")
}

// onMessage handles one message from the page, on the GUI thread.
func onMessage(line string) {
	var m struct {
		Type string `json:"type"`
	}
	if json.Unmarshal([]byte(line), &m) != nil {
		return
	}
	switch m.Type {
	case "ready":
		readyOnce.Do(func() { close(ready) })
		reply(line)
	case "hidden":
		hideWindow()
		reply(line)
	default:
		reply(line)
	}
}

// onHidden handles the compositor closing the surface.
func onHidden() { reply(`{"type":"hidden"}`) }
