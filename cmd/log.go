package cmd

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/procinfo"
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
			now := time.Now()
			cutoff := now.Add(-since)
			keep := func(ts time.Time, names []string) bool {
				return (since == 0 || !ts.Before(cutoff)) &&
					(name == "" || slices.Contains(names, name))
			}
			// Read everything: an expiry or a still running exec depends
			// on entries the filters may drop.
			all, bad, err := audit.Read(p.LogFile(), nil)
			if err != nil {
				return err
			}
			if bad > 0 {
				fmt.Fprintf(c.ErrOrStderr(), "skipped %d malformed lines in %s\n", bad, p.LogFile())
			}

			out := c.OutOrStdout()
			if asJSON {
				for _, l := range all {
					if !keep(l.TS, l.Names) {
						continue
					}
					if _, err := fmt.Fprintf(out, "%s\n", l.Raw); err != nil {
						return err
					}
				}
				return nil
			}
			running := stillRunning(all)
			var rows []logRow
			for i, l := range all {
				if keep(l.TS, l.Names) {
					rows = append(rows, entryRow(l, running[i]))
				}
			}
			var expired []logRow
			for _, r := range expiries(all, now) {
				if keep(r.ts, r.names) {
					expired = append(expired, r)
				}
			}
			rows = mergeRows(rows, expired)
			if len(rows) == 0 {
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "TIME\tACTION\tRESULT\tTIER\tNAMES\tDETAIL\tFROM")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					r.ts.Local().Format(time.DateTime), r.action, orDash(r.result), orDash(r.tier),
					orDash(strings.Join(r.names, ",")), orDash(r.detail), orDash(strings.Join(r.from, " < ")))
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

// logRow is one line of the log table: an entry, or an expiry worked out
// from one.
type logRow struct {
	ts                           time.Time
	action, result, tier, detail string
	names, from                  []string
}

func entryRow(l audit.Line, running bool) logRow {
	result := "ok"
	switch {
	case l.OK:
	case l.Status != nil:
		result = "exit " + strconv.Itoa(*l.Status)
	default:
		result = l.Error
	}
	detail := l.Cmd
	if l.For != "" {
		detail = "for " + l.For
	}
	if l.Took != "" {
		detail += " after " + l.Took
	}
	if running {
		detail += " (running)"
	}
	// ancestry[0] is broker itself; the rest is who asked.
	var from []string
	if len(l.Ancestry) > 1 {
		from = l.Ancestry[1:]
	}
	return logRow{ts: l.TS, action: string(l.Action), result: result, tier: l.Tier,
		names: l.Names, detail: detail, from: from}
}

// brokerAlive reports whether pid is a live broker other than this one.
// A var so tests can stand in for a process.
var brokerAlive = defaultBrokerAlive

func defaultBrokerAlive(pid int) bool {
	if pid <= 0 || pid == os.Getpid() {
		return false
	}
	self, other := procinfo.Ancestry(os.Getpid()), procinfo.Ancestry(pid)
	return len(self) > 0 && len(other) > 0 && other[0] == self[0]
}

// stillRunning marks, by index into lines, each exec that handed out its
// secrets, has no exit after it under the same pid, and whose broker is
// still alive. A pid reused by another broker could be marked too.
func stillRunning(lines []audit.Line) map[int]bool {
	exited := map[int]bool{}
	running := map[int]bool{}
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		switch {
		case l.Action == audit.Exit:
			exited[l.PID] = true
		case l.Action == audit.Exec && l.OK && !exited[l.PID] && brokerAlive(l.PID):
			running[i] = true
		}
	}
	return running
}

// expiries works out when each private grant in lines ran out: a grant
// or request entry's time plus its duration, for the names no later
// grant, request, revoke, or rm took over first. Grants still live at now
// have none. The kernel drops an expired grant on its own, with no broker
// running to log it, so these rows are computed, never written.
func expiries(lines []audit.Line, now time.Time) []logRow {
	type grant struct {
		at      time.Time
		ttl     string
		expires time.Time
	}
	live := map[string]grant{}
	var rows []logRow
	// end closes name's grant at t: an expiry row when it had run out by
	// then, nothing when t cut it short.
	ended := map[grant][]string{}
	end := func(name string, t time.Time) {
		g, ok := live[name]
		if !ok {
			return
		}
		delete(live, name)
		if !g.expires.After(t) {
			ended[g] = append(ended[g], name)
		}
	}
	for _, l := range lines {
		ok := l.OK && len(l.Names) > 0
		granted := ok && (l.Action == audit.Grant || l.Action == audit.Request) && l.For != ""
		dropped := ok && (l.Action == audit.Revoke || l.Action == audit.Rm)
		if !granted && !dropped {
			continue
		}
		ttl, err := time.ParseDuration(l.For)
		for _, n := range l.Names {
			end(n, l.TS)
			if granted && err == nil {
				live[n] = grant{at: l.TS, ttl: l.For, expires: l.TS.Add(ttl)}
			}
		}
	}
	for n := range live {
		end(n, now)
	}
	for g, names := range ended {
		slices.Sort(names)
		rows = append(rows, logRow{ts: g.expires, action: "expire", tier: "private",
			names: names, detail: "after " + g.ttl})
	}
	slices.SortFunc(rows, func(a, b logRow) int {
		if c := a.ts.Compare(b.ts); c != 0 {
			return c
		}
		return strings.Compare(strings.Join(a.names, ","), strings.Join(b.names, ","))
	})
	return rows
}

// mergeRows puts each computed row (sorted by time) in rows (in log
// order) before the first entry that is later than it.
func mergeRows(rows, computed []logRow) []logRow {
	if len(computed) == 0 {
		return rows
	}
	out := make([]logRow, 0, len(rows)+len(computed))
	for _, r := range rows {
		for len(computed) > 0 && computed[0].ts.Before(r.ts) {
			out = append(out, computed[0])
			computed = computed[1:]
		}
		out = append(out, r)
	}
	return append(out, computed...)
}
