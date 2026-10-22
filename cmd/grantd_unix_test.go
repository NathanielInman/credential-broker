//go:build linux || darwin

package cmd

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// TestGrantd runs the real grant agent the way macOS does: started by the
// first grant, detached, serving this BROKER_HOME's socket.
func TestGrantd(t *testing.T) {
	t.Setenv("BROKER_HOME", t.TempDir())
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	c := &keyring.Client{Socket: p.GrantSocket(), Start: keyring.StartAgent}
	if _, err := c.Read("API_TOKEN"); !errors.Is(err, keyring.ErrNotGranted) {
		t.Fatalf("Read before any agent: %v", err)
	}
	if _, err := os.Stat(p.GrantSocket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a read started an agent")
	}
	if _, err := c.Add("API_TOKEN", []byte("token-value"), time.Minute); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Revoke("API_TOKEN") })
	if v, err := c.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
	if fi, err := os.Stat(p.GrantSocket()); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("socket %v, %v; want it private", fi, err)
	}

	// A second agent finds the first serving and leaves.
	if err := keyring.StartAgent(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if v, err := c.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("Read after a second start = %q, %v", v, err)
	}
}
