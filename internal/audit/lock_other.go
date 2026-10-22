//go:build !linux && !darwin && !windows

package audit

import "os"

// lock is a no-op where broker refuses to run anyway.
func lock(*os.File) {}
