package procinfo

import (
	"bytes"
	"errors"

	"golang.org/x/sys/unix"
)

// parent reads pid's command name and parent with sysctl kern.proc.pid.
func parent(pid int) (string, int, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", 0, err
	}
	if int(kp.Proc.P_pid) != pid {
		return "", 0, errors.New("no such process")
	}
	comm := kp.Proc.P_comm[:]
	if i := bytes.IndexByte(comm, 0); i >= 0 {
		comm = comm[:i]
	}
	return string(comm), int(kp.Eproc.Ppid), nil
}

// execPath reads the path pid was started from out of kern.procargs2:
// argc as an int32, then the executable path, NUL-terminated. It is the
// path given to execve, so it may be relative.
func execPath(pid int) (string, error) {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return "", err
	}
	if len(buf) < 5 {
		return "", errors.New("short kern.procargs2")
	}
	path := buf[4:]
	if i := bytes.IndexByte(path, 0); i >= 0 {
		path = path[:i]
	}
	return string(path), nil
}
