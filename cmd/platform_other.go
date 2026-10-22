//go:build !linux

package cmd

import (
	"runtime"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
)

func checkPlatform() error {
	return agenterr.New(agenterr.Unsupported, "broker v2 supports linux only, not %s", runtime.GOOS)
}
