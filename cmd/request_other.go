//go:build !linux && !darwin && !windows

package cmd

import (
	"net"
	"os"
	"syscall"
	"time"
)

func newSession() *syscall.SysProcAttr { return nil }

func lockFile(string, time.Time) (func(), error) { return nil, errUnsupported }

func listenPrivate(string) (net.Listener, error) { return nil, errUnsupported }

var stopSignals = []os.Signal{os.Interrupt}
