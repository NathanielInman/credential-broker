package cmd

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func checkPlatform() error { return nil }

// harden makes this process non-dumpable, as ssh-agent does: no other
// process of the same user may ptrace it or read its memory.
func harden() error {
	return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}

// desktopSession reports why no window can open here, or nil.
func desktopSession() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return errors.New("no desktop session (WAYLAND_DISPLAY and DISPLAY are unset)")
	}
	return nil
}
