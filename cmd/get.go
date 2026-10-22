package cmd

import (
	"filippo.io/age"
	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Print a secret (private needs a fresh touch)",
		Long: "get prints a secret's value and a newline. Public secrets print at once.\n" +
			"Private secrets always need a fresh touch; a grant does not count, since\n" +
			"grants are for exec only.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			name := args[0]
			e := audit.Entry{Action: audit.Get, Names: args}
			defer func() { record(c, e, err) }()
			if err := vault.ValidateName(name); err != nil {
				return err
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			s, err := v.Get(name)
			if err != nil {
				return err
			}
			e.Tier = string(s.Tier)
			var id age.Identity
			if s.Tier == vault.Private {
				if err := requireTTY(c.InOrStdin(), "broker get "+name); err != nil {
					return err
				}
				if id, err = v.UnlockPrivate(vault.TerminalUI(c.ErrOrStderr())); err != nil {
					return err
				}
			}
			value, err := v.Decrypt(name, id)
			if err != nil {
				return err
			}
			defer clear(value)
			out := c.OutOrStdout()
			if _, err := out.Write(value); err != nil {
				return err
			}
			// set --stdin drops one trailing newline, so get | set round trips.
			_, err = out.Write([]byte("\n"))
			return err
		},
	}
}
