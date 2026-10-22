//go:build !linux && !darwin

package keyring

import (
	"errors"
	"net"
	"syscall"
)

func peerUID(net.Conn) (int, error) { return 0, errors.New("peer credentials are not supported here") }

func detached() *syscall.SysProcAttr { return nil }
