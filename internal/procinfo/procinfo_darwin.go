package procinfo

import (
	"bytes"
	"errors"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// parent reads pid's command name and parent with sysctl kern.proc.pid.
// The name is the one the process was started as (argv[0]) where macOS
// lets us read it: p_comm is the executable's file name, which for a
// versioned binary says "2.1.308" where argv[0] says "claude".
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
	name := string(comm)
	if a, err := argv0(pid); err == nil && a != "" {
		name = a
	}
	return name, int(kp.Eproc.Ppid), nil
}

// argv0 is the base name of pid's argv[0], from kern.procargs2: argc as an
// int32, the executable path, NUL padding, then argv. Only processes of
// the same user can be read.
func argv0(pid int) (string, error) {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return "", err
	}
	if len(buf) < 5 {
		return "", errors.New("short kern.procargs2")
	}
	rest := buf[4:]
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return "", errors.New("malformed kern.procargs2")
	}
	rest = bytes.TrimLeft(rest[i:], "\x00")
	if j := bytes.IndexByte(rest, 0); j >= 0 {
		rest = rest[:j]
	}
	return filepath.Base(string(rest)), nil
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
