package cmd

import "github.com/spf13/cobra"

func newSetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "set NAME",
		Short:   "Store a secret from a hidden prompt or stdin",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE:    notImplemented,
	}
	c.Flags().Bool("public", false, "store in the public tier (no touch, not redacted)")
	c.Flags().String("desc", "", "one-line description of what the secret is for")
	c.Flags().Bool("stdin", false, "read the value from stdin instead of a prompt")
	return c
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Delete a secret",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE:    notImplemented,
	}
}
