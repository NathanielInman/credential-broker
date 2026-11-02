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
	// Changed means a grant is still in place, but its value changed in
	// the vault since: exec refuses it, and a new grant is needed.
	Changed bool `json:"changed,omitempty"`
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
				var st grantState
				if krErr == nil {
					if st, err = lookupGrant(kr, v, s.Name); err != nil {
						return err
					}
				}
				e := lsEntry{Secret: s, Granted: st.Live, Changed: st.Changed}
				if st.Live {
					e.Expires = st.Expires
				}
				entries = append(entries, e)
			}

			out := c.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetEscapeHTML(false)
				return enc.Encode(entries)
			}
			if len(entries) == 0 {
				// On stderr, so stdout stays empty for scripts.
				fmt.Fprintln(c.ErrOrStderr(), "no secrets yet; add one with broker set NAME, or broker import FILE")
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
				case e.Changed:
					grant = "changed"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name, e.Tier, grant, e.Description)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	return c
}
