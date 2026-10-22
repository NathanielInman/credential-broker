package keyring

import (
	"errors"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// peerIsUs checks the process at the other end of conn runs as this user.
func peerIsUs(conn net.Conn) error {
	uid, err := peerUID(conn)
	if err != nil {
		return err
	}
	if uid != os.Getuid() {
		return errors.New("peer is another user")
	}
	return nil
}

// refused reports a socket file with nobody listening.
func refused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

// peerUID is the uid of the process at the other end of conn.
func peerUID(conn net.Conn) (int, error) {
	return sockopt(conn, func(fd int) (int, error) {
		cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			return 0, err
		}
		return int(cred.Uid), nil
	})
}

func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

func sockopt(conn net.Conn, f func(int) (int, error)) (int, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, err
	}
	var out int
	var ferr error
	if err := raw.Control(func(fd uintptr) { out, ferr = f(int(fd)) }); err != nil {
		return 0, err
	}
	return out, ferr
}
