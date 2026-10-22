//go:build darwin || windows

// Package keyringtest keeps tests that grant secrets away from the real
// grant store.
//
// On macOS and Windows grants live in a grant agent whose socket sits in
// broker's runtime directory, so a test with its own BROKER_HOME has its
// own agent and is isolated already.
package keyringtest

import (
	"errors"
	"testing"
)

// Main runs m: there is nothing to isolate beyond BROKER_HOME.
func Main(m *testing.M) int { return m.Run() }

func Isolated() bool { return true }

// Require never skips where grants live in the agent.
func Require(t testing.TB) { t.Helper() }

// InNewSession runs f: the grant agent is per user, not per session, so
// any process sees the same grants.
func InNewSession(f func() error) error { return f() }

func SessionKeyring() (int, error) { return 0, errors.New("no session keyrings here") }
