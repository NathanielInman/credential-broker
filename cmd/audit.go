package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// record appends e to the audit log with err as its outcome. Commands
// defer it with their named error result, so failures are logged too.
// Logging is best effort: when the line cannot be written, record warns
// on stderr and the command carries on as if it had been.
func record(c *cobra.Command, e audit.Entry, err error) {
	if err != nil {
		e.Error = string(agenterr.From(err).Code)
	} else {
		e.OK = true
	}
	p, werr := paths.Resolve()
	if werr == nil {
		werr = audit.Append(p.LogFile(), e)
	}
	if werr != nil {
		fmt.Fprintf(c.ErrOrStderr(), "broker: audit log not written: %v\n", werr)
	}
}

// tierOf is the tier the audit log records for names: theirs when they
// share one, "mixed" when they span both, and empty when v is nil or none
// of them is in it.
func tierOf(v *vault.Vault, names []string) string {
	if v == nil {
		return ""
	}
	var tier vault.Tier
	for _, name := range names {
		s, err := v.Get(name)
		switch {
		case err != nil:
		case tier == "":
			tier = s.Tier
		case tier != s.Tier:
			return "mixed"
		}
	}
	return string(tier)
}
