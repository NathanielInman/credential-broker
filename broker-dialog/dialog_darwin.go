package main

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework WebKit
import "C"

import "syscall"

func platformInit() {}

// redirectStdout points fd 1 at stderr.
func redirectStdout() error { return syscall.Dup2(2, 1) }
