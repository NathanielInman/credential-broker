package keyring

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

// Keyring is where Linux grants live: the kernel user keyring first, and
// the grant agent beside it. User keyrings are per user namespace, so a
// sandbox (bubblewrap, flatpak) sees an empty one; the agent's socket in
// broker's runtime directory reaches across, wherever the sandbox binds
// that directory. Every grant goes to both, with the same duration.
type Keyring struct {
	// kernel is nil where keyctl is unavailable (seccomp).
	kernel *kernel
	agent  *Client
}

// User returns the kernel user keyring together with this user's grant
// agent. It fails only when neither can be used.
func User() (*Keyring, error) {
	p, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	k := &Keyring{agent: &Client{Socket: p.GrantSocket(), Start: StartAgent}}
	// An agent started inside a sandbox dies with it and leaves the host
	// none the wiser, so a sandbox only uses one already running.
	if sandboxed() {
		k.agent.Start = nil
	}
	kern, err := openKernel()
	if err != nil {
		if k.agent.Start != nil {
			return nil, err
		}
		kern = nil
	}
	k.kernel = kern
	return k, nil
}

// sandboxed reports whether this process runs in a user namespace other
// than the initial one, whose uid_map covers every uid from 0. Tests
// replace it.
var sandboxed = func() bool {
	b, err := os.ReadFile("/proc/self/uid_map")
	if err != nil {
		return false
	}
	return strings.Join(strings.Fields(string(b)), " ") != "0 0 4294967295"
}

// noGrant reports an error that means no live grant.
func noGrant(err error) bool {
	return errors.Is(err, ErrNotGranted) || errors.Is(err, ErrExpired)
}

// Add grants name for ttl in the kernel keyring and the agent and returns
// when the kernel's grant expires. If the agent cannot take it, the kernel
// grant is revoked again, so sandboxes never miss a grant the host sees.
// A sandbox with no agent to reach grants in its own keyring only.
func (k *Keyring) Add(name string, value []byte, ttl time.Duration) (time.Time, error) {
	if k.kernel == nil {
		return k.agent.Add(name, value, ttl)
	}
	expires, err := k.kernel.Add(name, value, ttl)
	if err != nil {
		return time.Time{}, err
	}
	if _, err := k.agent.Add(name, value, ttl); err != nil {
		if k.agent.Start == nil && errors.Is(err, errNoAgent) {
			return expires, nil
		}
		_, _ = k.kernel.Revoke(name)
		return time.Time{}, err
	}
	return expires, nil
}

// Read returns name's granted value, ErrNotGranted, or ErrExpired.
func (k *Keyring) Read(name string) ([]byte, error) {
	var kerr error = ErrNotGranted
	if k.kernel != nil {
		v, err := k.kernel.Read(name)
		if !noGrant(err) {
			return v, err
		}
		kerr = err
	}
	v, err := k.agent.Read(name)
	if errors.Is(err, ErrNotGranted) {
		return nil, kerr
	}
	return v, err
}

// Lookup returns name's live grant, ErrNotGranted, or ErrExpired.
func (k *Keyring) Lookup(name string) (Grant, error) {
	var kerr error = ErrNotGranted
	if k.kernel != nil {
		g, err := k.kernel.Lookup(name)
		if !noGrant(err) {
			return g, err
		}
		kerr = err
	}
	g, err := k.agent.Lookup(name)
	if errors.Is(err, ErrNotGranted) {
		return Grant{}, kerr
	}
	return g, err
}

// List returns every live grant in either store, sorted by name, with the
// kernel's expiry where both hold one.
func (k *Keyring) List() ([]Grant, error) {
	byName := map[string]Grant{}
	if k.kernel != nil {
		gs, err := k.kernel.List()
		if err != nil {
			return nil, err
		}
		for _, g := range gs {
			byName[g.Name] = g
		}
	}
	gs, err := k.agent.List()
	if err != nil {
		return nil, err
	}
	for _, g := range gs {
		if _, ok := byName[g.Name]; !ok {
			byName[g.Name] = g
		}
	}
	out := make([]Grant, 0, len(byName))
	for _, g := range byName {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Revoke drops name's grant from both stores and reports whether either
// held a live one.
func (k *Keyring) Revoke(name string) (bool, error) {
	var live bool
	if k.kernel != nil {
		l, err := k.kernel.Revoke(name)
		if err != nil {
			return l, err
		}
		live = l
	}
	l, err := k.agent.Revoke(name)
	if err != nil {
		return live, fmt.Errorf("revoke %s: %w", name, err)
	}
	return live || l, nil
}
