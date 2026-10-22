package procinfo

import (
	"errors"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// parent reads pid's image name (without ".exe") and parent from a
// Toolhelp snapshot. Windows reuses pids and keeps a dead parent's pid in
// the entry, so a "parent" that started after pid is not its parent, and
// the walk stops there.
func parent(pid int) (string, int, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "", 0, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if int(e.ProcessID) != pid {
			continue
		}
		name := strings.TrimSuffix(windows.UTF16ToString(e.ExeFile[:]), ".exe")
		ppid := int(e.ParentProcessID)
		if ppid != 0 && !startedBefore(ppid, pid) {
			ppid = pid // ends the walk
		}
		return name, ppid, nil
	}
	return "", 0, errors.New("no such process")
}

// startedBefore reports whether process a started before process b.
func startedBefore(a, b int) bool {
	ta, err := created(a)
	if err != nil {
		return false
	}
	tb, err := created(b)
	return err == nil && ta <= tb
}

func created(pid int) (int64, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	var c, x, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &x, &k, &u); err != nil {
		return 0, err
	}
	return c.Nanoseconds(), nil
}

// Exe is the full path of pid's executable.
func Exe(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// Cwd would need reading pid's PEB; broker shows no directory on Windows.
func Cwd(int) (string, error) { return "", ErrUnsupported }
