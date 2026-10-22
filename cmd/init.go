package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func newInitCmd() *cobra.Command {
	var identities []string
	var allowPlain bool
	c := &cobra.Command{
		Use:     "init --identity FILE...",
		Short:   "Create the vault, wrapped to one or more unlock identities",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) (err error) {
			defer func() { record(c, audit.Entry{Action: audit.Init}, err) }()
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			v, err := vault.Init(p, identities, vault.InitOptions{
				AllowPlain: allowPlain,
				UI:         vault.TerminalUI(c.ErrOrStderr()),
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "created vault in %s\n", p.Data)
			printUnlocks(c.OutOrStdout(), v)
			return nil
		},
	}
	c.Flags().StringArrayVar(&identities, "identity", nil, "age identity file that unlocks the private tier (repeatable)")
	c.Flags().BoolVar(&allowPlain, "insecure-plain-identity", false, "allow plain AGE-SECRET-KEY identities (tests only: they protect nothing)")
	return c
}

// zoneNotes says what each unlock zone protects against, for init and
// restore to print.
var zoneNotes = map[config.Zone]string{
	config.ZoneHardware:   "the key stays on the device; unlocking needs it present",
	config.ZonePassphrase: "safe at rest, but anyone who learns the passphrase can unlock",
	config.ZonePlain:      "no protection: anything running as you can unlock, even without a terminal",
}

// printUnlocks lists the identities that unlock v's private tier and the
// protection that leaves the vault with.
func printUnlocks(out io.Writer, v *vault.Vault) {
	cfg := v.Config()
	fmt.Fprintln(out, "private tier unlocks with:")
	for _, u := range cfg.Unlock {
		fmt.Fprintf(out, "  %s (%s)\n", u.Path, u.Zone())
	}
	z := cfg.Zone()
	fmt.Fprintf(out, "protection: %s, %s\n", z, zoneNotes[z])
	if z != config.ZoneHardware && len(cfg.Unlock) > 1 {
		fmt.Fprintln(out, "the weakest unlock decides: any one of them opens the private tier")
	}
}
