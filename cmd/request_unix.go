//go:build linux || darwin

package cmd

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/NathanielInman/credential-broker/internal/approval"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// newSession detaches a child into a session of its own, so killing the
// requester's process group leaves it running.
func newSession() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// lockRequests takes the request lock, waiting until deadline for another
// window to close, and returns its release.
func lockRequests(p paths.Paths, deadline time.Time) (func(), error) {
	if err := paths.MkdirAll(p.Run); err != nil {
		return nil, err
	}
	release, err := lockFile(p.RequestLock(), deadline)
	if errors.Is(err, errLocked) {
		return nil, approval.ErrTimeout
	}
	return release, err
}

var errLocked = errors.New("locked by another process")

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
