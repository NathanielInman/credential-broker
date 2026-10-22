// Package keyringtest keeps tests that grant secrets away from the real
// kernel user keyring.
package keyringtest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

const envIsolated = "BROKER_TEST_KEYRING"

// Main reruns the test binary in a new user namespace that maps only the
// caller's uid and gid, from a fresh session keyring, and returns its exit
// status. User keyrings are per user namespace, so the copy gets an empty
// user keyring of its own: grants made by tests never reach the real one
// and vanish when the tests exit, even if they crash. Call it from
// TestMain. If user namespaces are unavailable, m runs directly and
// Require skips.
func Main(m *testing.M) int {
	if os.Getenv(envIsolated) != "" {
		return m.Run()
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "keyringtest:", err)
		return 1
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), envIsolated+"=isolated")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}},
		Pdeathsig:   syscall.SIGKILL,
	}

	var startErr error
	err = InNewSession(func() error {
		if startErr = cmd.Start(); startErr != nil {
			return startErr
		}
		return cmd.Wait()
	})
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	case startErr != nil:
		fmt.Fprintf(os.Stderr, "keyringtest: no user namespace (%v); keyring tests skip\n", startErr)
		os.Setenv(envIsolated, "unavailable")
		return m.Run()
	}
	fmt.Fprintln(os.Stderr, "keyringtest:", err)
	return 1
}

// Isolated reports whether Main isolated the user keyring.
func Isolated() bool { return os.Getenv(envIsolated) == "isolated" }

// Require skips t unless Main isolated the user keyring.
func Require(t testing.TB) {
	t.Helper()
	if !Isolated() {
		t.Skip("needs an isolated user keyring (keyringtest.Main in a user namespace)")
	}
}

// InNewSession runs f on an OS thread that has joined a new anonymous
// session keyring, so processes f starts run in a different session than
// the caller. The thread is retired afterwards.
func InNewSession(f func() error) error {
	errc := make(chan error, 1)
	go func() {
		// Never unlocked, so the runtime exits the thread with the goroutine.
		runtime.LockOSThread()
		if _, err := unix.KeyctlInt(unix.KEYCTL_JOIN_SESSION_KEYRING, 0, 0, 0, 0); err != nil {
			errc <- fmt.Errorf("join session keyring: %w", err)
			return
		}
		errc <- f()
	}()
	return <-errc
}

// SessionKeyring returns the calling thread's session keyring ID.
func SessionKeyring() (int, error) {
	return unix.KeyctlGetKeyringID(unix.KEY_SPEC_SESSION_KEYRING, true)
}
