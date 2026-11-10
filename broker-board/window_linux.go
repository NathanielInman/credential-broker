package main

// /* gtk4-layer-shell must come before libwayland-client when linking. */
// #cgo pkg-config: gtk4-layer-shell-0 gtk4 webkitgtk-6.0
// #include <stdlib.h>
// #include "window.h"
import "C"

import (
	"os"
	"syscall"
	"unsafe"
)

func platformInit() {
	// WebKit's DMA-BUF renderer trips a Wayland protocol error on NVIDIA
	// with explicit sync ("Missing acquire timeline").
	if _, ok := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !ok {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
	// The panel is a layer-shell surface, which only Wayland has.
	if _, ok := os.LookupEnv("GDK_BACKEND"); !ok {
		os.Setenv("GDK_BACKEND", "wayland")
	}
}

// protocolOut keeps the real stdout for the protocol and points fd 1 at
// stderr, so nothing GTK or WebKit prints can corrupt it.
func protocolOut() (*os.File, error) {
	fd, err := syscall.Dup(1)
	if err != nil {
		return nil, err
	}
	out := os.NewFile(uintptr(fd), "protocol")
	if err := syscall.Dup3(2, 1, 0); err != nil {
		return nil, err
	}
	return out, nil
}

func newWindow(html string) {
	s := C.CString(html)
	C.window_create(s)
	C.free(unsafe.Pointer(s))
}

func runWindow()  { C.window_run() }
func quitWindow() { C.window_quit() }
func hideWindow() { C.window_hide() }

func showWindow(monitor string) {
	s := C.CString(monitor)
	C.window_show(s)
	C.free(unsafe.Pointer(s))
}

func evalPage(js string) {
	s := C.CString(js)
	C.window_eval(s)
	C.free(unsafe.Pointer(s))
}

//export goMessage
func goMessage(s *C.char) { onMessage(C.GoString(s)) }

//export goHidden
func goHidden() { onHidden() }
