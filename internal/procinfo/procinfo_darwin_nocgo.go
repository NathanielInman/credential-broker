//go:build darwin && !cgo

package procinfo

// Exe is the path pid was started from. Without cgo there is no
// proc_pidpath, so it may be relative.
func Exe(pid int) (string, error) { return execPath(pid) }

// Cwd needs proc_pidinfo, which needs cgo.
func Cwd(int) (string, error) { return "", ErrUnsupported }
