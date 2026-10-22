//go:build !linux

package cmd

import (
	"errors"
	"syscall"
	"time"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

var errUnsupported = errors.New("requests need linux")

func harden() error                    { return errUnsupported }
func newSession() *syscall.SysProcAttr { return nil }

func lockRequests(paths.Paths, time.Time) (func(), error) { return nil, errUnsupported }
