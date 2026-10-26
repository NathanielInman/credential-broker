package keyring

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/keyring/keyringtest"
)

// testStore is an isolated kernel keyring with a fresh grant agent beside
// it.
func testStore(t *testing.T) (*Keyring, *kernel, *Client) {
	t.Helper()
	kern := testKeyring(t)
	c, _ := agent(t, time.Minute)
	return &Keyring{kernel: kern, agent: c}, kern, c
}

func TestStoreAddGoesToBoth(t *testing.T) {
	k, kern, c := testStore(t)
	expires, err := k.Add("API_TOKEN", []byte("token-value"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := kern.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("kernel Read = %q, %v", v, err)
	}
	g, err := c.Lookup("API_TOKEN")
	if err != nil {
		t.Fatalf("agent Lookup: %v", err)
	}
	if d := g.Expires.Sub(expires); d < 0 || d > time.Second {
		t.Fatalf("agent expires %s, kernel %s", g.Expires, expires)
	}

	live, err := k.Revoke("API_TOKEN")
	if err != nil || !live {
		t.Fatalf("Revoke = %v, %v", live, err)
	}
	if _, err := kern.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("kernel Read after revoke: %v", err)
	}
	if _, err := c.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("agent Read after revoke: %v", err)
	}
}

// A grant made in another user namespace reaches this one only through
// the agent.
func TestStoreFallsBackToAgent(t *testing.T) {
	k, _, c := testStore(t)
	if _, err := c.Add("FROM_HOST", []byte("host-value"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Add("LOCAL", []byte("local-value"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("FROM_HOST"); err != nil || string(v) != "host-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
	if g, err := k.Lookup("FROM_HOST"); err != nil || g.Expires.IsZero() {
		t.Fatalf("Lookup = %+v, %v", g, err)
	}
	gs, err := k.List()
	if err != nil || len(gs) != 2 || gs[0].Name != "FROM_HOST" || gs[1].Name != "LOCAL" {
		t.Fatalf("List = %+v, %v", gs, err)
	}
	if _, err := k.Read("NEVER"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read of an ungranted name: %v", err)
	}
	if live, err := k.Revoke("FROM_HOST"); err != nil || !live {
		t.Fatalf("Revoke = %v, %v", live, err)
	}
	if _, err := k.Read("FROM_HOST"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read after revoke: %v", err)
	}
}

func TestStoreExpiresInBoth(t *testing.T) {
	k, _, _ := testStore(t)
	if _, err := k.Add("SHORT", []byte("v"), time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := k.Read("SHORT"); !noGrant(err) {
		t.Fatalf("Read after expiry: %v", err)
	}
}

// A sandbox with no agent to reach still grants in its own keyring.
func TestStoreSandboxWithoutAgent(t *testing.T) {
	kern := testKeyring(t)
	k := &Keyring{kernel: kern, agent: &Client{Socket: filepath.Join(t.TempDir(), "none.sock")}}
	if _, err := k.Add("API_TOKEN", []byte("token-value"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
}

// Outside a sandbox an agent that will not start fails the grant, and the
// kernel keeps nothing a sandbox could not see.
func TestStoreAgentFailureUndoesGrant(t *testing.T) {
	kern := testKeyring(t)
	k := &Keyring{kernel: kern, agent: &Client{
		Socket: filepath.Join(t.TempDir(), "none.sock"),
		Start:  func() error { return errors.New("no agent today") },
	}}
	if _, err := k.Add("API_TOKEN", []byte("token-value"), time.Hour); err == nil {
		t.Fatal("Add succeeded without an agent")
	}
	if _, err := kern.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("kernel kept the grant: %v", err)
	}
}

// Where keyctl is blocked the agent alone holds grants.
func TestStoreAgentOnly(t *testing.T) {
	c, _ := agent(t, time.Minute)
	k := &Keyring{agent: c}
	if _, err := k.Add("API_TOKEN", []byte("token-value"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
}

func TestSandboxed(t *testing.T) {
	keyringtest.Require(t)
	if !sandboxed() {
		t.Fatal("tests run in their own user namespace but sandboxed() is false")
	}
}
