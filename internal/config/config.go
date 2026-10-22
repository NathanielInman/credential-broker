// Package config reads and writes broker's config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

// Unlock is one identity that can unwrap the private tier identity: a
// plugin stub (Identity), or a plain or passphrase file (Recipient only).
type Unlock struct {
	// Path is the identity file given to init.
	Path string `json:"path"`
	// Identity is a plugin identity stub (AGE-PLUGIN-...). Stubs only point
	// at hardware, so they are kept here and unlock works without Path.
	Identity string `json:"identity,omitempty"`
	// Recipient belongs to a plain or passphrase identity; the key itself
	// is never copied, and unlocking reads it from Path again. With a
	// plugin stub it is the recipient the plugin wrote beside the stub
	// ("# public key: age1se1..."), for plugins that cannot derive one.
	Recipient string `json:"recipient,omitempty"`
	// Passphrase marks a Path that is encrypted with a passphrase (age -p).
	Passphrase bool `json:"passphrase,omitempty"`
}

// Zone is how well an unlock identity protects the private tier.
type Zone string

const (
	// ZoneHardware is an age plugin identity: the key stays on the device
	// and unlocking needs it present (a touch, for a YubiKey).
	ZoneHardware Zone = "hardware"
	// ZonePassphrase is an identity file encrypted with a passphrase:
	// safe at rest, but the passphrase alone unlocks it.
	ZonePassphrase Zone = "passphrase"
	// ZonePlain is a plain AGE-SECRET-KEY file: anything running as the
	// user can read it and unlock.
	ZonePlain Zone = "plain"
)

// Plugin is the age plugin a hardware unlock uses ("yubikey", "se"), or
// "" for any other unlock. A stub is Bech32 with the HRP
// "AGE-PLUGIN-<NAME>-", and the data alphabet has no '1', so the last '1'
// ends the HRP.
func (u Unlock) Plugin() string {
	i := strings.LastIndexByte(u.Identity, '1')
	if u.Identity == "" || i < 0 {
		return ""
	}
	name := strings.TrimSuffix(strings.TrimPrefix(u.Identity[:i], "AGE-PLUGIN-"), "-")
	return strings.ToLower(name)
}

// zoneRank orders zones from weakest to strongest.
var zoneRank = map[Zone]int{ZonePlain: 0, ZonePassphrase: 1, ZoneHardware: 2}

// Zone reports how u protects the private tier.
func (u Unlock) Zone() Zone {
	switch {
	case u.Identity != "":
		return ZoneHardware
	case u.Passphrase:
		return ZonePassphrase
	default:
		return ZonePlain
	}
}

// Zone reports the weakest zone among c's unlock identities. Any one of
// them unwraps the private tier, so the weakest is what the vault has.
func (c *Config) Zone() Zone {
	weakest := ZoneHardware
	for _, u := range c.Unlock {
		if z := u.Zone(); zoneRank[z] < zoneRank[weakest] {
			weakest = z
		}
	}
	return weakest
}

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
