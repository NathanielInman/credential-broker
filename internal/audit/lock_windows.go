package audit

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lock takes an exclusive LockFileEx lock on f, or gives up after
// lockWait.
func lock(f *os.File) {
	deadline := time.Now().Add(lockWait)
	for {
		var ol windows.Overlapped
		err := windows.LockFileEx(windows.Handle(f.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
		if err == nil || !errors.Is(err, windows.ERROR_LOCK_VIOLATION) || time.Now().After(deadline) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
