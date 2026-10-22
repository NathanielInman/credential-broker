package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

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
		RunE: func(c *cobra.Command, _ []string) error {
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
			out := c.OutOrStdout()
			fmt.Fprintf(out, "created vault in %s\nprivate tier unlocks with:\n", p.Data)
			for _, u := range v.Config().Unlock {
				kind := "plugin"
				if u.Plain() {
					kind = "plain, no touch"
				}
				fmt.Fprintf(out, "  %s (%s)\n", u.Path, kind)
			}
			return nil
		},
	}
	c.Flags().StringArrayVar(&identities, "identity", nil, "age identity file that unlocks the private tier (repeatable)")
	c.Flags().BoolVar(&allowPlain, "insecure-plain-identity", false, "allow plain AGE-SECRET-KEY identities (tests, machines without hardware)")
	return c
}
