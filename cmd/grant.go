package cmd

import "github.com/spf13/cobra"

func newGrantCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "grant NAME... | --all",
		Short:   "Let exec use private secrets for a while (one touch)",
		GroupID: groupHuman,
		RunE:    notImplemented,
	}
	c.Flags().Duration("for", 0, "grant duration (default from config, max 24h)")
	c.Flags().Bool("all", false, "grant every private secret")
	c.Flags().Bool("force", false, "allow a duration over 24h")
	return c
}

func newRevokeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "revoke [NAME...] | --all",
		Short:   "Drop grants before they expire",
		GroupID: groupHuman,
		RunE:    notImplemented,
	}
	c.Flags().Bool("all", false, "revoke every grant")
	return c
}
