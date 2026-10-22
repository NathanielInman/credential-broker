//go:build !linux

package audit

import "os"

// lock is a no-op off linux, where broker refuses to run anyway.
func lock(*os.File) {}
