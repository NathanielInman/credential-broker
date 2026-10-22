//go:build linux || darwin || windows

package keyring

import (
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// agent serves a grant agent on a fresh socket and returns a client for
// it. The socket lives in a short temp dir to stay under sun_path.
func agent(t *testing.T, idle time.Duration) (*Client, <-chan error) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "g.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Serve(ln, idle) }()
	t.Cleanup(func() { ln.Close() })
	return &Client{Socket: sock}, done
}

func TestAgentAddReadRevoke(t *testing.T) {
	c, _ := agent(t, time.Minute)
	expires, err := c.Add("API_TOKEN", []byte("token-value"), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(expires); d < 2*time.Hour-5*time.Second || d > 2*time.Hour+time.Second {
		t.Fatalf("expires in %s", d)
	}
	if v, err := c.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
	if g, err := c.Lookup("API_TOKEN"); err != nil || !g.Expires.Equal(expires) {
		t.Fatalf("Lookup = %+v, %v", g, err)
	}
	if _, err := c.Add("DB_PASS", []byte("db"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if gs, err := c.List(); err != nil || len(gs) != 2 || gs[0].Name != "API_TOKEN" || gs[1].Name != "DB_PASS" {
		t.Fatalf("List = %+v, %v", gs, err)
	}

	// Replacing a grant keeps the newest value.
	if _, err := c.Add("API_TOKEN", []byte("new-value"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Read("API_TOKEN"); string(v) != "new-value" {
		t.Fatalf("Read after replace = %q", v)
	}

	if live, err := c.Revoke("API_TOKEN"); err != nil || !live {
		t.Fatalf("Revoke = %v, %v", live, err)
	}
	if live, _ := c.Revoke("API_TOKEN"); live {
		t.Error("second Revoke reported a live grant")
	}
	if _, err := c.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read after revoke: %v", err)
	}
}

func TestAgentExpiry(t *testing.T) {
	c, _ := agent(t, time.Minute)
	if _, err := c.Add("API_TOKEN", []byte("token-value"), time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2100 * time.Millisecond)
	if _, err := c.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read after expiry: %v", err)
	}
	if gs, _ := c.List(); len(gs) != 0 {
		t.Fatalf("List after expiry = %+v", gs)
	}
}

func TestAgentExitsWhenIdle(t *testing.T) {
	c, done := agent(t, 400*time.Millisecond)
	if _, err := c.Add("API_TOKEN", []byte("v"), time.Second); err != nil {
		t.Fatal(err)
	}
	// Holding a grant keeps it up past idle.
	select {
	case <-done:
		t.Fatal("agent exited while holding a grant")
	case <-time.After(700 * time.Millisecond):
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not exit once idle")
	}
	// With no agent, there are no grants, and asking starts nothing.
	if _, err := c.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read with no agent: %v", err)
	}
	if gs, err := c.List(); err != nil || gs != nil {
		t.Fatalf("List with no agent = %v, %v", gs, err)
	}
}

func TestAgentStartsForAdd(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "g.sock")
	starts := 0
	c := &Client{Socket: sock, Start: func() error {
		starts++
		ln, err := net.Listen("unix", sock)
		if err != nil {
			return err
		}
		t.Cleanup(func() { ln.Close() })
		go Serve(ln, time.Minute)
		return nil
	}}
	if _, err := c.Read("X"); !errors.Is(err, ErrNotGranted) || starts != 0 {
		t.Fatalf("Read: %v, %d starts", err, starts)
	}
	if _, err := c.Add("X", []byte("v"), time.Minute); err != nil || starts != 1 {
		t.Fatalf("Add: %v, %d starts", err, starts)
	}
	if v, err := c.Read("X"); err != nil || string(v) != "v" {
		t.Fatalf("Read = %q, %v", v, err)
	}
}
