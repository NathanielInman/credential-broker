package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/audit"
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
	c.Flags().BoolVar(&allowPlain, "insecure-plain-identity", false, "allow plain AGE-SECRET-KEY identities (tests, machines without hardware)")
	return c
}

// printUnlocks lists the identities that unlock v's private tier.
func printUnlocks(out io.Writer, v *vault.Vault) {
	fmt.Fprintln(out, "private tier unlocks with:")
	for _, u := range v.Config().Unlock {
		kind := "plugin"
		if u.Plain() {
			kind = "plain, no touch"
		}
		fmt.Fprintf(out, "  %s (%s)\n", u.Path, kind)
	}
}
