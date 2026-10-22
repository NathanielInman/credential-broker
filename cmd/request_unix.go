//go:build linux || darwin

package cmd

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

// newSession detaches a child into a session of its own, so killing the
// requester's process group leaves it running.
func newSession() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// lockFile takes an exclusive flock on file, waiting until deadline, and
// returns its release.
func lockFile(file string, deadline time.Time) (func(), error) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, paths.FileMode)
	if err != nil {
		return nil, err
	}
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() { f.Close() }, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, errLocked
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// listenPrivate listens on a unix socket only this user can connect to.
func listenPrivate(sock string) (net.Listener, error) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	return net.Listen("unix", sock)
}

// stopSignals are the signals that shut the grant agent down.
var stopSignals = []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP}
