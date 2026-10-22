package cmd

import (
	"errors"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

func checkPlatform() error { return nil }

// harden refuses debugger attachment. The real protection on macOS is the
// hardened runtime (codesign -o runtime), under which no other process of
// the same user may read this one's memory; PT_DENY_ATTACH backs it up.
func harden() error { return unix.PtraceDenyAttach() }

// desktopSession reports why no window can open here, or nil. Only the
// login session that owns the screen (launchd's "Aqua" manager) can show
// one; an ssh session is "Background" or "StandardIO".
func desktopSession() error {
	out, err := exec.Command("launchctl", "managername").Output()
	if err != nil {
		return nil // assume a desktop rather than refuse on a launchctl quirk
	}
	if m := strings.TrimSpace(string(out)); m != "Aqua" {
		return errors.New("not in the desktop login session (launchctl managername is " + m + ")")
	}
	return nil
}
