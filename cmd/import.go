package cmd

import "github.com/spf13/cobra"

func newImportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "import FILE",
		Short:   "Move NAME=value lines from a shell env file into the vault",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE:    notImplemented,
	}
	c.Flags().Bool("public", false, "store imported secrets in the public tier")
	return c
}
