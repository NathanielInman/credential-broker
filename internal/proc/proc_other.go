//go:build !linux

package proc

import (
	"errors"
	"os/exec"
)

// Run is unavailable off linux.
func Run(*exec.Cmd) (int, error) {
	return 0, &StartError{Err: errors.New("exec is linux only")}
}
