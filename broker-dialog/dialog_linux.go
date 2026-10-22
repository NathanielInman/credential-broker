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

// redirectStdout points fd 1 at stderr.
func redirectStdout() error { return syscall.Dup3(2, 1, 0) }
