package main

// #cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
import "C"

import (
	"os"
	"syscall"
)

func platformInit() {
	// WebKit's DMA-BUF renderer trips a Wayland protocol error on NVIDIA
	// with explicit sync ("Missing acquire timeline"). A dialog does not
	// need it.
	if _, ok := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !ok {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
}

// protocolOut keeps the real stdout for the protocol and points fd 1 at
// stderr, so nothing GTK, WebKit, or Cocoa prints can corrupt it.
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
