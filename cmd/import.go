package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/envfile"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// maxEnvFile is the largest file import reads. Shell env files are tiny;
// this only stops a mistaken path from filling memory.
const maxEnvFile = 4 << 20

func newImportCmd() *cobra.Command {
	var public, dryRun, overwrite bool
	c := &cobra.Command{
		Use:   "import FILE",
		Short: "Move NAME=value lines from a shell env file into the vault",
		Long: "import reads export NAME=value and NAME=value lines from a shell env file\n" +
			"such as ~/.zshenv, stores each value, and prints the lines that can now\n" +
			"be deleted. It never prints values and never edits FILE. Values the shell\n" +
			"would have to work out ($VAR, $(cmd), ~) are skipped and reported, as are\n" +
			"names already in the vault (unless --overwrite) and invalid names.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			file := args[0]
			e := audit.Entry{Action: audit.Import}
			if !dryRun {
				defer func() { record(c, e, err) }()
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			src, err := readEnvFile(file)
			if err != nil {
				return err
			}
			p, err := planImport(v, envfile.Parse(src), public, overwrite)
			if err != nil {
				return err
			}
			if !dryRun {
				for i, s := range p.secrets {
					if _, err = v.Set(s.name, []byte(s.value), vault.SetOptions{Tier: s.tier}); err != nil {
						p.secrets = p.secrets[:i]
						break
					}
					e.Names = append(e.Names, s.name)
				}
				e.Tier = tierOf(v, e.Names)
			}
			if werr := p.print(c.OutOrStdout(), file, dryRun); err == nil {
				err = werr
			}
			return err
		},
	}
	c.Flags().BoolVar(&public, "public", false, "store imported secrets in the public tier")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be imported without storing anything")
	c.Flags().BoolVar(&overwrite, "overwrite", false, "replace secrets already in the vault")
	return c
}

func readEnvFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEnvFile+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxEnvFile {
		return "", agenterr.New(agenterr.Usage, "%s is larger than %d bytes; is it a shell env file?", file, maxEnvFile)
	}
	return string(data), nil
}

// plan is what import does with a file.
type plan struct {
	secrets []importSecret
	skipped []skippedLine
}

// importSecret is one name to store: the value of its last assignment.
// Every assignment of it is deletable once it is stored, unless the file
// still expands it somewhere.
type importSecret struct {
	name  string
	value string
	tier  vault.Tier
	spans []span
	refs  []int
}

type skippedLine struct {
	line         int
	name, reason string
}

// span is the lines one statement covers.
type span struct{ first, last int }

// planImport decides what to store and what to skip. A name's last
// assignment wins, as in the shell: when the shell would have to work it
// out, every assignment of the name is skipped.
func planImport(v *vault.Vault, f envfile.File, public, overwrite bool) (plan, error) {
	var p plan
	reasons := make([]string, len(f.Assignments))
	last := map[string]int{}
	for i, a := range f.Assignments {
		switch {
		case a.Reason != "":
			reasons[i] = a.Reason
		case vault.ValidateName(a.Name) != nil:
			reasons[i] = "invalid name: must be A-Z, 0-9, and _, not starting with a digit"
		default:
			if err := vault.ValidateValue([]byte(a.Value)); err != nil {
				reasons[i] = agenterr.From(err).Message
			}
		}
		last[a.Name] = i
	}

	index := map[string]int{}
	for i, a := range f.Assignments {
		reason := reasons[i]
		if l := last[a.Name]; reason == "" && reasons[l] != "" {
			reason = fmt.Sprintf("set again on line %d", f.Assignments[l].First)
		}
		if reason != "" {
			p.skipped = append(p.skipped, skippedLine{a.First, a.Name, reason})
			continue
		}
		j, ok := index[a.Name]
		if !ok {
			j = len(p.secrets)
			index[a.Name] = j
			p.secrets = append(p.secrets, importSecret{name: a.Name, refs: f.Refs[a.Name]})
		}
		p.secrets[j].value = a.Value
		p.secrets[j].spans = append(p.secrets[j].spans, span{a.First, a.Last})
	}

	// Check the vault: names already in it are skipped unless overwritten,
	// and an overwrite keeps the tier unless --public moves it, as set does.
	secrets := p.secrets[:0]
	for _, s := range p.secrets {
		existing, err := v.Get(s.name)
		var aerr *agenterr.Error
		switch {
		case err == nil && !overwrite:
			for _, sp := range s.spans {
				p.skipped = append(p.skipped, skippedLine{sp.first, s.name, "already in the vault (--overwrite replaces it)"})
			}
			continue
		case err == nil:
			s.tier = existing.Tier
		case errors.As(err, &aerr) && aerr.Code == agenterr.UnknownSecret:
			s.tier = vault.Private
		default:
			return plan{}, err
		}
		if public {
			s.tier = vault.Public
		}
		secrets = append(secrets, s)
	}
	p.secrets = secrets
	slices.SortStableFunc(p.skipped, func(a, b skippedLine) int { return a.line - b.line })
	return p, nil
}

// print reports the plan by name and line number. Never values.
func (p plan) print(w io.Writer, file string, dryRun bool) error {
	verb := "imported"
	if dryRun {
		verb = "would import"
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	var deletable []span
	var kept []importSecret
	if len(p.secrets) == 0 {
		fmt.Fprintf(tw, "nothing %s from %s\n", verb, file)
	} else {
		fmt.Fprintf(tw, "%s (%d):\n", verb, len(p.secrets))
	}
	for _, s := range p.secrets {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.name, s.tier, lineList(s.spans))
		if len(s.refs) > 0 {
			kept = append(kept, s)
		} else {
			deletable = append(deletable, s.spans...)
		}
	}
	if len(p.skipped) > 0 {
		fmt.Fprintf(tw, "skipped (%d):\n", len(p.skipped))
		for _, s := range p.skipped {
			fmt.Fprintf(tw, "  line %d\t%s\t%s\n", s.line, s.name, s.reason)
		}
	}
	if len(kept) > 0 {
		fmt.Fprintf(tw, "kept, still used in %s (%d):\n", file, len(kept))
		for _, s := range kept {
			fmt.Fprintf(tw, "  %s\t%s\tused on %s\n", lineList(s.spans), s.name, lineList(lineSpans(s.refs)))
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(deletable) == 0 {
		return nil
	}
	prefix := ""
	if dryRun {
		prefix = "after import, "
	}
	_, err := fmt.Fprintf(w, "%ssafe to delete from %s: %s\n", prefix, file, lineList(deletable))
	return err
}

func lineSpans(lines []int) []span {
	spans := make([]span, len(lines))
	for i, l := range lines {
		spans[i] = span{l, l}
	}
	return spans
}

// lineList formats spans as "line 3" or "lines 3, 5-7", merging spans that
// touch.
func lineList(spans []span) string {
	spans = slices.Clone(spans)
	slices.SortFunc(spans, func(a, b span) int { return a.first - b.first })
	var merged []span
	for _, s := range spans {
		if n := len(merged); n > 0 && s.first <= merged[n-1].last+1 {
			merged[n-1].last = max(merged[n-1].last, s.last)
			continue
		}
		merged = append(merged, s)
	}
	parts := make([]string, len(merged))
	for i, s := range merged {
		parts[i] = strconv.Itoa(s.first)
		if s.last != s.first {
			parts[i] += "-" + strconv.Itoa(s.last)
		}
	}
	if len(merged) == 1 && merged[0].first == merged[0].last {
		return "line " + parts[0]
	}
	return "lines " + strings.Join(parts, ", ")
}
