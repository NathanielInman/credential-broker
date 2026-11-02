package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// maxGrant is the longest grant allowed without --force.
const maxGrant = 24 * time.Hour

// requireTTY guards commands that unlock the private tier. Tests swap it
// out, since their plain unlock identity never prompts.
var requireTTY = agenterr.RequireTTY

func newGrantCmd() *cobra.Command {
	var all, force bool
	var dur time.Duration
	c := &cobra.Command{
		Use:     "grant NAME... | --all",
		Short:   "Let exec use private secrets for a while (one touch)",
		GroupID: groupHuman,
		RunE: func(c *cobra.Command, names []string) (err error) {
			e := audit.Entry{Action: audit.Grant, Names: names}
			defer func() { record(c, e, err) }()
			if all == (len(names) > 0) {
				return agenterr.New(agenterr.Usage, "grant takes NAME... or --all")
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			ttl, err := grantDuration(v.Config(), c.Flags().Changed("for"), dur, force)
			if err != nil {
				return err
			}
			e.For = shortDuration(ttl)
			secrets, err := grantable(c, v, names, all)
			if err != nil {
				return err
			}
			if len(secrets) == 0 {
				// Only public secrets, or none at all: nothing was granted,
				// so the line says so instead of naming a grant.
				e.Names, e.For = nil, ""
				return nil
			}
			// Log what is granted: only private secrets, and with --all,
			// each of them by name.
			e.Names, e.Tier = nil, string(vault.Private)
			for _, s := range secrets {
				e.Names = append(e.Names, s.Name)
			}

			ask := "broker grant " + strings.Join(names, " ")
			if all {
				ask = "broker grant --all"
			}
			if err := requireTTY(c.InOrStdin(), ask+" --for "+shortDuration(ttl)); err != nil {
				return err
			}
			kr, err := userKeyring()
			if err != nil {
				return err
			}
			id, err := v.UnlockPrivate(vault.TerminalUI(c.ErrOrStderr()))
			if err != nil {
				return err
			}
			for _, s := range secrets {
				expires, err := grantSecret(kr, v, s.Name, id, ttl)
				if err != nil {
					return err
				}
				fmt.Fprintf(c.OutOrStdout(), "granted %s for %s (until %s)\n",
					s.Name, shortDuration(ttl), expires.Local().Format(time.DateTime))
			}
			return nil
		},
	}
	c.Flags().DurationVar(&dur, "for", 0, "grant duration (default from config, max 24h)")
	c.Flags().BoolVar(&all, "all", false, "grant every private secret")
	c.Flags().BoolVar(&force, "force", false, "allow a duration over 24h")
	return c
}

// grantDuration is --for when given, else the config's default_grant,
// capped at maxGrant unless force.
func grantDuration(cfg *config.Config, given bool, dur time.Duration, force bool) (time.Duration, error) {
	if !given {
		def := cfg.DefaultGrant
		if def == "" {
			def = config.DefaultGrant
		}
		var err error
		if dur, err = time.ParseDuration(def); err != nil {
			return 0, fmt.Errorf("config default_grant: %w", err)
		}
	}
	switch {
	case dur <= 0:
		return 0, agenterr.New(agenterr.Usage, "grant duration %s is not positive", dur)
	case dur > maxGrant && !force:
		return 0, agenterr.New(agenterr.Usage, "grant duration %s is over %s; pass --force to allow it", shortDuration(dur), shortDuration(maxGrant))
	}
	return dur, nil
}

// grantable resolves names (or every secret with all) to the private
// secrets to grant. Public secrets need no grant and are skipped with a
// note; unknown names fail before any touch.
func grantable(c *cobra.Command, v *vault.Vault, names []string, all bool) ([]vault.Secret, error) {
	var secrets []vault.Secret
	if all {
		var err error
		if secrets, err = v.List(); err != nil {
			return nil, err
		}
	} else {
		var unknown []string
		for _, name := range names {
			s, err := v.Get(name)
			if err != nil {
				if agenterr.From(err).Code == agenterr.UnknownSecret {
					unknown = append(unknown, name)
					continue
				}
				return nil, err
			}
			secrets = append(secrets, s)
		}
		if len(unknown) > 0 {
			return nil, &agenterr.Error{Code: agenterr.UnknownSecret, Names: unknown}
		}
	}

	var out []vault.Secret
	for _, s := range secrets {
		if s.Tier == vault.Private {
			out = append(out, s)
		} else if !all {
			fmt.Fprintf(c.ErrOrStderr(), "%s is public and needs no grant\n", s.Name)
		}
	}
	if len(out) == 0 {
		fmt.Fprintln(c.ErrOrStderr(), "no private secrets to grant")
	}
	return out, nil
}

// shortDuration formats d without trailing zero units: 1h, not 1h0m0s.
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func newRevokeCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:     "revoke NAME... | --all",
		Short:   "Drop grants before they expire",
		GroupID: groupHuman,
		RunE: func(c *cobra.Command, names []string) (err error) {
			e := audit.Entry{Action: audit.Revoke, Names: names}
			defer func() { record(c, e, err) }()
			if all == (len(names) > 0) {
				return agenterr.New(agenterr.Usage, "revoke takes NAME... or --all")
			}
			for _, name := range names {
				if err := vault.ValidateName(name); err != nil {
					return err
				}
			}
			kr, err := userKeyring()
			if err != nil {
				return err
			}
			if all {
				grants, err := kr.List()
				if err != nil {
					return err
				}
				for _, g := range grants {
					names = append(names, g.Name)
				}
				e.Names = names
			}

			// The log names what was revoked, with its tier when the vault
			// still knows it (revoke works without one).
			e.Names = nil
			v, _ := openVault()
			defer func() { e.Tier = tierOf(v, e.Names) }()
			out := c.OutOrStdout()
			for _, name := range names {
				live, err := kr.Revoke(name)
				if err != nil {
					return err
				}
				if live {
					e.Names = append(e.Names, name)
					fmt.Fprintf(out, "revoked %s\n", name)
				} else if !all {
					fmt.Fprintf(out, "%s was not granted\n", name)
				}
			}
			if all && len(names) == 0 {
				fmt.Fprintln(out, "no grants to revoke")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "revoke every grant")
	return c
}

// grantState is what exec can do with a secret's grant.
type grantState struct {
	// Live means exec can use the grant.
	Live bool
	// Changed means the grant has not expired but holds a value the vault
	// has since replaced (broker set, import, restore), so it is not Live:
	// exec would hand out the old value.
	Changed bool
	// Expires is when a live or changed grant ends, nil when unknown.
	Expires *time.Time
}

// lookupGrant reports the state of name's grant in kr. Expired and missing
// grants both read as not granted. A grant from before grants carried the
// vault's Version cannot be checked, and counts as live.
func lookupGrant(kr *keyring.Keyring, v *vault.Vault, name string) (grantState, error) {
	g, err := kr.Lookup(name)
	if errors.Is(err, keyring.ErrNotGranted) || errors.Is(err, keyring.ErrExpired) {
		return grantState{}, nil
	}
	if err != nil {
		return grantState{}, err
	}
	var st grantState
	if !g.Expires.IsZero() {
		exp := g.Expires.UTC()
		st.Expires = &exp
	}
	if g.Version != "" {
		now, err := v.Version(name)
		if err != nil {
			return grantState{}, err
		}
		if now != g.Version {
			st.Changed = true
			return st, nil
		}
	}
	st.Live = true
	return st, nil
}

// grantSecret decrypts name with the unlocked private identity and grants
// it for ttl, along with the vault's Version of it, so exec can tell when
// the vault changes under the grant.
func grantSecret(kr *keyring.Keyring, v *vault.Vault, name string, id age.Identity, ttl time.Duration) (time.Time, error) {
	// Version before Decrypt: a change in between then reads as a changed
	// grant, never as a current one (see vault.Version).
	version, err := v.Version(name)
	if err != nil {
		return time.Time{}, err
	}
	value, err := v.Decrypt(name, id)
	if err != nil {
		return time.Time{}, err
	}
	defer clear(value)
	return kr.Add(name, value, version, ttl)
}
