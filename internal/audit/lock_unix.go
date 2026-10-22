//go:build linux || darwin

package audit

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// lock takes an exclusive flock on f, or gives up after lockWait.
func lock(f *os.File) {
	deadline := time.Now().Add(lockWait)
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil || !errors.Is(err, unix.EWOULDBLOCK) || time.Now().After(deadline) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
