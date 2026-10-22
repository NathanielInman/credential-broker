package cmd

import "github.com/spf13/cobra"

func newLsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "ls",
		Short:   "List secrets, tiers, descriptions, and grants (never values)",
		GroupID: groupAgent,
		Args:    usageArgs(cobra.NoArgs),
		RunE:    notImplemented,
	}
	c.Flags().Bool("json", false, "machine-readable output")
	return c
}
