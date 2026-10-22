//go:build !linux && !darwin

package cmd

import (
	"errors"
	"runtime"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
)

func checkPlatform() error {
	return agenterr.New(agenterr.Unsupported, "broker v2 supports linux and macOS, not %s", runtime.GOOS)
}

func harden() error         { return errUnsupported }
func desktopSession() error { return errUnsupported }

var errUnsupported = errors.New("not supported on " + runtime.GOOS)
