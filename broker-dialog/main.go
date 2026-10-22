// Command broker-dialog is broker's request window, the desktop dialog in
// which a human approves an agent's request for secrets.
//
// broker runs it and speaks JSON lines with it: messages to show arrive on
// stdin and go to the page as they are; what the human does comes back on
// stdout. The page (ui.html) holds all the presentation and none of the
// decisions, and nothing it shows is ever parsed as HTML.
//
// It is a separate program so broker itself stays pure Go, with no GTK or
// WebKit to link.
package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <stdlib.h>
#include "window.h"
*/
import "C"

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed ui.html
var page string

const (
	width  = 470
	height = 560
	// maxMessage bounds one line from broker.
	maxMessage = 1 << 20
)

var (
	// proto is the real stdout. fd 1 is pointed at stderr, so nothing GTK
	// or WebKit prints can corrupt the protocol.
	proto *os.File
	send  sync.Mutex
	// replied is set once the window has said deny, so closing it after
	// does not say it again.
	replied bool

	ready     = make(chan struct{})
	readyOnce sync.Once
)

func init() {
	// GTK must run on the main thread.
	runtime.LockOSThread()
}

func main() {
	fd, err := syscall.Dup(1)
	if err != nil {
		os.Exit(1)
	}
	proto = os.NewFile(uintptr(fd), "protocol")
	if err := syscall.Dup3(2, 1, 0); err != nil {
		os.Exit(1)
	}

	// WebKit's DMA-BUF renderer trips a Wayland protocol error on NVIDIA
	// with explicit sync ("Missing acquire timeline"). A dialog does not
	// need it.
	if _, ok := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !ok {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	html := C.CString(page)
	C.window_create(html, width, height)
	C.free(unsafe.Pointer(html))
	go pump()
	C.window_run()
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
		js := C.CString("broker.receive(" + string(lit) + ")")
		C.window_eval(js)
		C.free(unsafe.Pointer(js))
	}
	C.window_quit()
}

// reply forwards one line from the page to broker.
func reply(line string) {
	send.Lock()
	defer send.Unlock()
	_, _ = proto.WriteString(line + "\n")
}

//export goMessage
func goMessage(s *C.char) {
	line := C.GoString(s)
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
		C.window_resize(C.int(m.Height))
	case "exit":
		C.window_quit()
	case "deny":
		replied = true
		reply(line)
	default:
		reply(line)
	}
}

//export goClosed
func goClosed() {
	if !replied {
		replied = true
		reply(`{"type":"deny"}`)
	}
}
