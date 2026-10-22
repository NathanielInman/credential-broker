package cmd

import "github.com/spf13/cobra"

func newBackupCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "backup -o FILE",
		Short:   "Write a passphrase-encrypted copy of the vault",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE:    notImplemented,
	}
	c.Flags().StringP("output", "o", "", "backup file to write")
	c.Flags().Bool("generate-passphrase", false, "generate a 7 word diceware passphrase")
	return c
}

func newRestoreCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "restore FILE",
		Short:   "Rebuild the vault from a backup",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE:    notImplemented,
	}
	c.Flags().StringArray("identity", nil, "wrap the private tier to this identity instead of the backup's recipients (repeatable)")
	c.Flags().Bool("force", false, "overwrite an existing vault")
	return c
}
