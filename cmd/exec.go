package cmd

import (
	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
)

func newExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "exec NAME... -- CMD [ARGS...]",
		Short:   "Run CMD with secrets in its environment, redacting its output",
		GroupID: groupAgent,
		Args:    execArgs,
		RunE:    notImplemented,
	}
}

func execArgs(c *cobra.Command, args []string) error {
	dash := c.ArgsLenAtDash()
	if dash < 1 || dash == len(args) {
		return agenterr.New(agenterr.Usage, "usage: broker exec NAME... -- CMD [ARGS...]")
	}
	return nil
}
