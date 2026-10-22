package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/vault"
)

// lsEntry is one secret in ls output. Never values.
type lsEntry struct {
	vault.Secret
	Granted bool       `json:"granted"`
	Expires *time.Time `json:"expires"`
}

func newLsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:     "ls",
		Short:   "List secrets, tiers, descriptions, and grants (never values)",
		GroupID: groupAgent,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			v, err := openVault()
			if err != nil {
				return err
			}
			secrets, err := v.List()
			if err != nil {
				return err
			}
			entries := make([]lsEntry, 0, len(secrets))
			for _, s := range secrets {
				granted, expires := grantStatus(s)
				entries = append(entries, lsEntry{Secret: s, Granted: granted, Expires: expires})
			}

			out := c.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetEscapeHTML(false)
				return enc.Encode(entries)
			}
			if len(entries) == 0 {
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tTIER\tGRANT\tDESCRIPTION")
			for _, e := range entries {
				grant := "-"
				if e.Granted && e.Expires != nil {
					grant = time.Until(*e.Expires).Round(time.Second).String()
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name, e.Tier, grant, e.Description)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	return c
}

// grantStatus reports whether s has a live grant and when it expires.
// Grants live in the kernel keyring, which the grant task wires in; until
// then nothing is ever granted.
func grantStatus(vault.Secret) (granted bool, expires *time.Time) {
	return false, nil
}
