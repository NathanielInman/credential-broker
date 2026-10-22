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

// harden makes this process non-dumpable, as ssh-agent does: no other
// process of the same user may ptrace it or read its memory.
func harden() error {
	return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}

// newSession detaches a child into a session of its own, so killing the
// requester's process group leaves it running.
func newSession() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// lockRequests takes the request lock, waiting until deadline for another
// window to close, and returns its release.
func lockRequests(p paths.Paths, deadline time.Time) (func(), error) {
	if err := paths.MkdirAll(p.Run); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p.RequestLock(), os.O_RDWR|os.O_CREATE, paths.FileMode)
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
			return nil, approval.ErrTimeout
		}
		time.Sleep(100 * time.Millisecond)
	}
}
