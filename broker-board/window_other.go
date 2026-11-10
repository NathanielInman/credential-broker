//go:build !linux

package main

import (
	"fmt"
	"os"
)

// The panel is a Wayland layer-shell surface; macOS (a menu bar extra) and
// Windows (a tray flyout) are planned.

func platformInit() {
	fmt.Fprintln(os.Stderr, "broker-board: the Signal Box panel runs on Linux with Wayland for now")
	os.Exit(1)
}

func protocolOut() (*os.File, error)    { return os.Stdout, nil }
func newWindow(string)                  {}
func runWindow()                        {}
func quitWindow()                       {}
func hideWindow()                       {}
func showWindow(float64, float64, bool) {}
func evalPage(string)                   {}
