//go:build !linux

package keyringtest

import (
	"errors"
	"testing"
)

// Main runs m directly; there is no kernel keyring to isolate.
func Main(m *testing.M) int { return m.Run() }

func Isolated() bool { return false }

// Require always skips off linux.
func Require(t testing.TB) {
	t.Helper()
	t.Skip("the kernel keyring is linux only")
}

func InNewSession(func() error) error { return errors.New("the kernel keyring is linux only") }

func SessionKeyring() (int, error) { return 0, errors.New("the kernel keyring is linux only") }
