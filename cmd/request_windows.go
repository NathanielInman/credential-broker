package cmd

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

// newSession starts a child in a process group of its own with a hidden
// console: closing or Ctrl+C in the requester's console leaves it running,
// and the age plugins it runs get no window of their own.
func newSession() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
}

// lockFile takes an exclusive LockFileEx lock on file, waiting until
// deadline, and returns its release.
func lockFile(file string, deadline time.Time) (func(), error) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, paths.FileMode)
	if err != nil {
		return nil, err
	}
	for {
		var ol windows.Overlapped
		err := windows.LockFileEx(windows.Handle(f.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
		if err == nil {
			return func() { f.Close() }, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
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

// listenPrivate listens on a unix socket. Connecting needs write access to
// the socket file, which inherits the runtime directory's ACL: under the
// user's profile, that is the user, SYSTEM, and Administrators. The agent
// also checks each peer's user.
func listenPrivate(sock string) (net.Listener, error) { return net.Listen("unix", sock) }

// stopSignals are the signals that shut the grant agent down: Ctrl+C,
// Ctrl+Break, and the console closing.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
