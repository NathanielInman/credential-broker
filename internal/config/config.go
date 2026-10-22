// Package config reads and writes broker's config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

// DefaultGrant is the grant duration init writes to a new config.
const DefaultGrant = "1h"

// Config is everything broker needs that is not a secret. It holds the tier
// recipients, so set can encrypt without unlocking anything.
type Config struct {
	// DefaultGrant is the duration grant uses without --for, as a Go
	// duration string.
	DefaultGrant string `json:"default_grant"`
	// PublicRecipient and PrivateRecipient are the age recipients of the
	// two tier identities.
	PublicRecipient  string `json:"public_recipient"`
	PrivateRecipient string `json:"private_recipient"`
	// Unlock lists the identities private.key.age is wrapped to.
	Unlock []Unlock `json:"unlock"`
}

// Unlock is one identity that can unwrap the private tier identity. Exactly
// one of Identity and Recipient is set.
type Unlock struct {
	// Path is the identity file given to init.
	Path string `json:"path"`
	// Identity is a plugin identity stub (AGE-PLUGIN-...). Stubs only point
	// at hardware, so they are kept here and unlock works without Path.
	Identity string `json:"identity,omitempty"`
	// Recipient belongs to a plain identity. The key itself is never
	// copied: unlocking reads it from Path again.
	Recipient string `json:"recipient,omitempty"`
}

// Plain reports whether u is a plain AGE-SECRET-KEY identity.
func (u Unlock) Plain() bool { return u.Identity == "" }

// Load reads the config at path. A missing file returns an error matching
// os.ErrNotExist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

// Save atomically writes c to path at paths.FileMode.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return paths.WriteFile(path, append(data, '\n'))
}
