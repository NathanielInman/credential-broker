package cmd

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "init --identity FILE...",
		Short:   "Create the vault, wrapped to one or more unlock identities",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE:    notImplemented,
	}
	c.Flags().StringArray("identity", nil, "age identity file that unlocks the private tier (repeatable)")
	c.Flags().Bool("insecure-plain-identity", false, "allow plain AGE-SECRET-KEY identities (tests, machines without hardware)")
	return c
}
