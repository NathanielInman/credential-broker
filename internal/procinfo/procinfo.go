// Package procinfo reads facts about other processes from the kernel: the
// executable, working directory, and chain of ancestors. broker uses them
// to show and log who asked for a secret, rather than trusting what the
// asking process says about itself.
package procinfo

import "errors"

// ErrUnsupported means this platform, or this build, cannot read the fact.
var ErrUnsupported = errors.New("not supported on this platform")

// maxDepth guards the ancestry walk against a cycle from reused pids.
const maxDepth = 128

// Ancestry returns the command names of pid and each of its ancestors up
// to pid 1: ["broker","bash","claude",...]. It stops early, keeping what it
// has, if a process exits mid-walk, and is nil where processes cannot be
// read.
func Ancestry(pid int) []string {
	var chain []string
	for depth := 0; pid > 0 && depth < maxDepth; depth++ {
		comm, ppid, err := parent(pid)
		if err != nil {
			break
		}
		chain = append(chain, comm)
		if ppid == pid {
			break
		}
		pid = ppid
	}
	return chain
}
