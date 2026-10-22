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
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, err
	}
	var uid int
	var ferr error
	if err := raw.Control(func(fd uintptr) {
		var cred *unix.Xucred
		if cred, ferr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED); ferr == nil {
			uid = int(cred.Uid)
		}
	}); err != nil {
		return 0, err
	}
	return uid, ferr
}

func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
