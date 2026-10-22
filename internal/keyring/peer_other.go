//go:build !linux && !darwin && !windows

package keyring

import (
	"errors"
	"net"
	"syscall"
)

func peerIsUs(net.Conn) error { return errors.New("peer credentials are not supported here") }

func refused(error) bool { return false }

func detached() *syscall.SysProcAttr { return nil }
