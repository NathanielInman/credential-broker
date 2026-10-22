package main

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework WebKit
import "C"

import (
	"os"
	"syscall"
)

func platformInit() {}

// protocolOut keeps the real stdout for the protocol and points fd 1 at
// stderr, so nothing GTK, WebKit, or Cocoa prints can corrupt it.
func protocolOut() (*os.File, error) {
	fd, err := syscall.Dup(1)
	if err != nil {
		return nil, err
	}
	out := os.NewFile(uintptr(fd), "protocol")
	if err := syscall.Dup2(2, 1); err != nil {
		return nil, err
	}
	return out, nil
}
