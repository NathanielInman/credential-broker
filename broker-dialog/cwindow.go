//go:build linux || darwin

package main

/*
#include <stdlib.h>
#include "window.h"
*/
import "C"

import "unsafe"

// cWindow is the C window in window_linux.c or window_darwin.m.
type cWindow struct{}

func newWindow(html string, width, height int) window {
	s := C.CString(html)
	C.window_create(s, C.int(width), C.int(height))
	C.free(unsafe.Pointer(s))
	return cWindow{}
}

func (cWindow) run()         { C.window_run() }
func (cWindow) resize(h int) { C.window_resize(C.int(h)) }
func (cWindow) quit()        { C.window_quit() }
func (cWindow) eval(js string) {
	s := C.CString(js)
	C.window_eval(s)
	C.free(unsafe.Pointer(s))
}

//export goMessage
func goMessage(s *C.char) { onMessage(C.GoString(s)) }

//export goClosed
func goClosed() { onClosed() }
