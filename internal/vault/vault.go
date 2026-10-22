// Package vault stores secrets as age files in two tiers.
//
// Each tier has its own X25519 identity. The public tier's identity sits in
// public.key, so public secrets decrypt without a prompt. The private tier's
// identity is wrapped to the unlock identities (usually a YubiKey) in
// private.key.age, so reading a private secret needs a touch while writing
// one only needs the private recipient from config. Every file is plain age,
// recoverable with the stock age CLI.
package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

type Tier string

const (
	Public  Tier = "public"
	Private Tier = "private"
)

// Tiers lists every tier.
var Tiers = []Tier{Private, Public}

// MaxValue is the longest value set accepts. It matches the kernel's
// MAX_ARG_STRLEN, the most exec could ever put in one env var.
const MaxValue = 128 << 10

var nameRE = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidateName checks that name is a usable environment variable name.
// Names become file names, so this also keeps paths inside the vault.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return agenterr.New(agenterr.Usage, "invalid secret name %q: must match %s", name, nameRE)
	}
	return nil
}

// Secret is a secret's metadata. Values are only returned by Decrypt.
type Secret struct {
	Name        string    `json:"name"`
	Tier        Tier      `json:"tier"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// meta is the <NAME>.json file beside each value.
type meta struct {
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// Vault is an initialized vault on disk.
type Vault struct {
	paths paths.Paths
	cfg   *config.Config
}

// Open loads the vault at p, or returns NoVault if init has not run.
func Open(p paths.Paths) (*Vault, error) {
	cfg, err := config.Load(p.ConfigFile())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &agenterr.Error{
			Code:     agenterr.NoVault,
			Message:  "no vault in " + p.Data,
			AskHuman: "broker init --identity FILE",
		}
	}
	if err != nil {
		return nil, err
	}
	return &Vault{paths: p, cfg: cfg}, nil
}

// Paths returns where the vault lives.
func (v *Vault) Paths() paths.Paths { return v.paths }

// Config returns the loaded config. Callers that change it must Save it.
func (v *Vault) Config() *config.Config { return v.cfg }

// InitOptions configures Init.
type InitOptions struct {
	// AllowPlain accepts plain AGE-SECRET-KEY unlock identities, which
	// unlock without a touch (--insecure-plain-identity).
	AllowPlain bool
	// UI handles plugin prompts.
	UI *plugin.ClientUI
}

// Init creates a vault at p whose private tier unlocks with the identities
// in identityFiles. It refuses to touch an existing vault.
func Init(p paths.Paths, identityFiles []string, opts InitOptions) (*Vault, error) {
	if len(identityFiles) == 0 {
		return nil, agenterr.New(agenterr.Usage, "init needs at least one --identity FILE")
	}
	for _, f := range []string{p.ConfigFile(), p.PublicKey(), p.PrivateKey()} {
		if _, err := os.Lstat(f); err == nil {
			return nil, agenterr.New(agenterr.VaultExists, "a vault already exists (%s)", f)
		}
	}

	var unlocks []config.Unlock
	seen := map[config.Unlock]bool{}
	for _, f := range identityFiles {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		us, _, err := readIdentityFile(abs, opts.UI)
		if err != nil {
			return nil, err
		}
		for _, u := range us {
			if u.Plain() && !opts.AllowPlain {
				return nil, agenterr.New(agenterr.Usage,
					"%s holds a plain AGE-SECRET-KEY identity, which unlocks without a touch; pass --insecure-plain-identity to allow it", abs)
			}
			if !seen[u] {
				seen[u] = true
				unlocks = append(unlocks, u)
			}
		}
	}

	pub, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, err
	}
	priv, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, err
	}
	// Wrap before writing anything, so a failing plugin leaves no
	// half-made vault behind.
	wrapped, err := wrapPrivate(identityFile(priv, Private), unlocks, opts.UI)
	if err != nil {
		return nil, err
	}

	if err := p.Ensure(); err != nil {
		return nil, err
	}
	for _, t := range Tiers {
		if err := paths.MkdirAll(p.TierDir(string(t))); err != nil {
			return nil, err
		}
	}
	if err := paths.WriteFile(p.PublicKey(), identityFile(pub, Public)); err != nil {
		return nil, err
	}
	if err := paths.WriteFile(p.PrivateKey(), wrapped); err != nil {
		return nil, err
	}
	cfg := &config.Config{
		DefaultGrant:     config.DefaultGrant,
		PublicRecipient:  pub.Recipient().String(),
		PrivateRecipient: priv.Recipient().String(),
		Unlock:           unlocks,
	}
	// Config last: Open treats it as the marker of a finished init.
	if err := cfg.Save(p.ConfigFile()); err != nil {
		return nil, err
	}
	return &Vault{paths: p, cfg: cfg}, nil
}

// UnlockPrivate unwraps the private tier identity. With a hardware unlock
// identity this is the touch. Never cache or persist the result.
func (v *Vault) UnlockPrivate(ui *plugin.ClientUI) (age.Identity, error) {
	ids, skipped := unlockIdentities(v.cfg.Unlock, ui)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no usable unlock identity: %w", skipped)
	}
	data, err := decryptFile(v.paths.PrivateKey(), ids...)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("unwrap %s: %w", v.paths.PrivateKey(), err), skipped)
	}
	id, err := parseTierIdentity(data, v.cfg.PrivateRecipient)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", v.paths.PrivateKey(), err)
	}
	return id, nil
}

// PublicIdentity reads the public tier identity from public.key.
func (v *Vault) PublicIdentity() (age.Identity, error) {
	data, err := os.ReadFile(v.paths.PublicKey())
	if err != nil {
		return nil, err
	}
	id, err := parseTierIdentity(data, v.cfg.PublicRecipient)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", v.paths.PublicKey(), err)
	}
	return id, nil
}

// Get returns name's metadata, or UnknownSecret.
func (v *Vault) Get(name string) (Secret, error) {
	if err := ValidateName(name); err != nil {
		return Secret{}, err
	}
	tiers, err := v.locate(name)
	if err != nil {
		return Secret{}, err
	}
	switch len(tiers) {
	case 0:
		return Secret{}, &agenterr.Error{Code: agenterr.UnknownSecret, Names: []string{name}}
	case 1:
		return v.secret(name, tiers[0])
	default:
		return Secret{}, inBothTiers(name)
	}
}

// List returns every secret's metadata, sorted by name.
func (v *Vault) List() ([]Secret, error) {
	var out []Secret
	for _, t := range Tiers {
		entries, err := os.ReadDir(v.paths.TierDir(string(t)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".age")
			if !ok || !e.Type().IsRegular() || ValidateName(name) != nil {
				continue
			}
			s, err := v.secret(name, t)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Tier < out[j].Tier
	})
	return out, nil
}

// Decrypt returns name's value. Public secrets use public.key; private
// secrets need private, the identity from UnlockPrivate.
func (v *Vault) Decrypt(name string, private age.Identity) ([]byte, error) {
	s, err := v.Get(name)
	if err != nil {
		return nil, err
	}
	id := private
	if s.Tier == Public {
		if id, err = v.PublicIdentity(); err != nil {
			return nil, err
		}
	} else if id == nil {
		return nil, fmt.Errorf("%s is private: decrypting it needs the unlocked private identity", name)
	}
	return decryptFile(v.valuePath(name, s.Tier), id)
}

// SetOptions configures Set.
type SetOptions struct {
	// Tier moves the secret to this tier. Empty keeps an existing secret's
	// tier; new secrets default to Private.
	Tier Tier
	// Description replaces the description when non-nil.
	Description *string
}

// Set stores value under name, encrypted to its tier's recipient. It never
// needs to unlock anything.
func (v *Vault) Set(name string, value []byte, opts SetOptions) (Secret, error) {
	if err := ValidateName(name); err != nil {
		return Secret{}, err
	}
	if err := validateValue(value); err != nil {
		return Secret{}, err
	}
	if opts.Description != nil && strings.ContainsAny(*opts.Description, "\r\n") {
		return Secret{}, agenterr.New(agenterr.Usage, "description must be one line")
	}
	if opts.Tier != "" && opts.Tier != Public && opts.Tier != Private {
		return Secret{}, fmt.Errorf("unknown tier %q", opts.Tier)
	}
	existing, err := v.locate(name)
	if err != nil {
		return Secret{}, err
	}

	tier := opts.Tier
	var m meta
	switch {
	case len(existing) > 1 && tier == "":
		return Secret{}, inBothTiers(name)
	case len(existing) > 0:
		if tier == "" {
			tier = existing[0]
		}
		if m, err = v.readMeta(name, existing[0]); err != nil {
			return Secret{}, err
		}
	case tier == "":
		tier = Private
	}
	now := time.Now().UTC().Truncate(time.Second)
	if m.Created.IsZero() {
		m.Created = now
	}
	m.Updated = now
	if opts.Description != nil {
		m.Description = *opts.Description
	}

	r, err := age.ParseX25519Recipient(v.recipient(tier))
	if err != nil {
		return Secret{}, fmt.Errorf("%s recipient in config: %w", tier, err)
	}
	ct, err := encrypt(value, r)
	if err != nil {
		return Secret{}, err
	}
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Secret{}, err
	}
	if err := paths.MkdirAll(v.paths.TierDir(string(tier))); err != nil {
		return Secret{}, err
	}
	if err := paths.WriteFile(v.valuePath(name, tier), ct); err != nil {
		return Secret{}, err
	}
	if err := paths.WriteFile(v.metaPath(name, tier), append(mj, '\n')); err != nil {
		return Secret{}, err
	}
	// The new copy is complete; only now drop the old tier's copy.
	for _, t := range existing {
		if t != tier {
			if err := v.remove(name, t); err != nil {
				return Secret{}, err
			}
		}
	}
	return Secret{Name: name, Tier: tier, Description: m.Description, Created: m.Created, Updated: m.Updated}, nil
}

// Remove deletes name from every tier it is in, or returns UnknownSecret.
func (v *Vault) Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	tiers, err := v.locate(name)
	if err != nil {
		return err
	}
	if len(tiers) == 0 {
		return &agenterr.Error{Code: agenterr.UnknownSecret, Names: []string{name}}
	}
	for _, t := range tiers {
		if err := v.remove(name, t); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(value []byte) error {
	switch {
	case len(value) == 0:
		return agenterr.New(agenterr.Usage, "value is empty")
	case len(value) > MaxValue:
		return agenterr.New(agenterr.Usage, "value is longer than %d bytes", MaxValue)
	case bytes.IndexByte(value, 0) >= 0:
		return agenterr.New(agenterr.Usage, "value contains a NUL byte, which cannot go in an environment variable")
	}
	return nil
}

// locate returns the tiers holding name. More than one means a tier move
// was interrupted.
func (v *Vault) locate(name string) ([]Tier, error) {
	var found []Tier
	for _, t := range Tiers {
		_, err := os.Lstat(v.valuePath(name, t))
		if err == nil {
			found = append(found, t)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return found, nil
}

func (v *Vault) secret(name string, t Tier) (Secret, error) {
	m, err := v.readMeta(name, t)
	if err != nil {
		return Secret{}, err
	}
	return Secret{Name: name, Tier: t, Description: m.Description, Created: m.Created, Updated: m.Updated}, nil
}

// readMeta reads name's metadata. A missing file is empty metadata, so a
// value copied in by hand still works.
func (v *Vault) readMeta(name string, t Tier) (meta, error) {
	var m meta
	data, err := os.ReadFile(v.metaPath(name, t))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("%s: %w", v.metaPath(name, t), err)
	}
	return m, nil
}

func (v *Vault) remove(name string, t Tier) error {
	for _, f := range []string{v.valuePath(name, t), v.metaPath(name, t)} {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (v *Vault) recipient(t Tier) string {
	if t == Public {
		return v.cfg.PublicRecipient
	}
	return v.cfg.PrivateRecipient
}

func (v *Vault) valuePath(name string, t Tier) string {
	return filepath.Join(v.paths.TierDir(string(t)), name+".age")
}

func (v *Vault) metaPath(name string, t Tier) string {
	return filepath.Join(v.paths.TierDir(string(t)), name+".json")
}

func inBothTiers(name string) error {
	return fmt.Errorf("%s is in both tiers (interrupted move?); run broker set %s with --public or --private to repair", name, name)
}
