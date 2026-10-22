//go:build linux || darwin

package cmd

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// TestGrantdSignal checks a signalled agent removes its socket, as an idle
// one does.
func TestGrantdSignal(t *testing.T) {
	t.Setenv("BROKER_HOME", t.TempDir())
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := keyring.AgentCommand()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, err := os.Stat(p.GrantSocket()); err != nil; _, err = os.Stat(p.GrantSocket()) {
		if time.Now().After(deadline) {
			t.Fatal("agent never listened")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("agent exit: %v", err)
	}
	if _, err := os.Stat(p.GrantSocket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket left behind: %v", err)
	}
}
