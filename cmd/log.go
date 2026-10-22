package cmd

import (
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func newLogCmd() *cobra.Command {
	var since time.Duration
	var name string
	var asJSON bool
	c := &cobra.Command{
		Use:     "log",
		Short:   "Show the audit log",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			if since < 0 {
				return agenterr.New(agenterr.Usage, "--since %s is negative", since)
			}
			if name != "" {
				if err := vault.ValidateName(name); err != nil {
					return err
				}
			}
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-since)
			lines, bad, err := audit.Read(p.LogFile(), func(e audit.Entry) bool {
				return (since == 0 || !e.TS.Before(cutoff)) &&
					(name == "" || slices.Contains(e.Names, name))
			})
			if err != nil {
				return err
			}
			if bad > 0 {
				fmt.Fprintf(c.ErrOrStderr(), "skipped %d malformed lines in %s\n", bad, p.LogFile())
			}

			out := c.OutOrStdout()
			if asJSON {
				for _, l := range lines {
					if _, err := fmt.Fprintf(out, "%s\n", l.Raw); err != nil {
						return err
					}
				}
				return nil
			}
			if len(lines) == 0 {
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "TIME\tACTION\tRESULT\tTIER\tNAMES\tDETAIL\tFROM")
			for _, l := range lines {
				result := "ok"
				if !l.OK {
					result = l.Error
				}
				detail := l.Cmd
				if l.For != "" {
					detail = "for " + l.For
				}
				// ancestry[0] is broker itself; the rest is who asked.
				var from []string
				if len(l.Ancestry) > 1 {
					from = l.Ancestry[1:]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					l.TS.Local().Format(time.DateTime), l.Action, orDash(result), orDash(l.Tier),
					orDash(strings.Join(l.Names, ",")), orDash(detail), orDash(strings.Join(from, " < ")))
			}
			return tw.Flush()
		},
	}
	c.Flags().DurationVar(&since, "since", 0, "only entries newer than this (e.g. 24h)")
	c.Flags().StringVar(&name, "name", "", "only entries for this secret")
	c.Flags().BoolVar(&asJSON, "json", false, "print raw JSON lines")
	return c
}

// orDash stands in for an empty table cell.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
