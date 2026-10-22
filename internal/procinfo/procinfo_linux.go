package procinfo

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

// parent reads pid's command name and parent from /proc/<pid>/stat.
func parent(pid int) (string, int, error) {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, err
	}
	return parseStat(stat)
}

// parseStat reads comm and ppid from a /proc/<pid>/stat line:
// "pid (comm) state ppid ...". comm may itself hold spaces and
// parentheses, so it runs to the last ')'.
func parseStat(stat []byte) (string, int, error) {
	open := bytes.IndexByte(stat, '(')
	end := bytes.LastIndexByte(stat, ')')
	if open < 0 || end < open {
		return "", 0, fmt.Errorf("malformed stat %q", stat)
	}
	fields := bytes.Fields(stat[end+1:])
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("malformed stat %q", stat)
	}
	ppid, err := strconv.Atoi(string(fields[1]))
	if err != nil {
		return "", 0, fmt.Errorf("malformed stat %q", stat)
	}
	return string(stat[open+1 : end]), ppid, nil
}

// Exe is the path of pid's executable.
func Exe(pid int) (string, error) { return os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)) }

// Cwd is pid's working directory.
func Cwd(pid int) (string, error) { return os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)) }
