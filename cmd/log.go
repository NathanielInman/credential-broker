package cmd

import "github.com/spf13/cobra"

func newLogCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "log",
		Short:   "Show the audit log",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE:    notImplemented,
	}
	c.Flags().Duration("since", 0, "only entries newer than this")
	c.Flags().String("name", "", "only entries for this secret")
	c.Flags().Bool("json", false, "print raw JSON lines")
	return c
}
