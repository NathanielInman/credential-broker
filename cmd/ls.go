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
			// Without the keyring (a sandbox that blocks keyctl, say) ls
			// still lists secrets, reporting nothing as granted.
			kr, krErr := userKeyring()
			if krErr != nil {
				fmt.Fprintf(c.ErrOrStderr(), "grants not shown: %v\n", krErr)
			}
			entries := make([]lsEntry, 0, len(secrets))
			for _, s := range secrets {
				var granted bool
				var expires *time.Time
				if krErr == nil {
					if granted, expires, err = lookupGrant(kr, s.Name); err != nil {
						return err
					}
				}
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
				switch {
				case e.Granted && e.Expires != nil:
					grant = shortDuration(time.Until(*e.Expires).Round(time.Second))
				case e.Granted:
					grant = "granted"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name, e.Tier, grant, e.Description)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	return c
}
