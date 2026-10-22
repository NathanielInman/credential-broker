package cmd

import "github.com/spf13/cobra"

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get NAME",
		Short:   "Print a secret (private needs a fresh touch)",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE:    notImplemented,
	}
}
