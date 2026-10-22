package keyring

import (
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// Keyring is the grant agent: macOS has no kernel keyring, so grants live
// in the memory of broker grantd (see agent.go).
type Keyring = Client

// User returns a client for this user's grant agent, which it starts on
// the first grant.
func User() (*Keyring, error) {
	p, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	return &Client{Socket: p.GrantSocket(), Start: StartAgent}, nil
}
