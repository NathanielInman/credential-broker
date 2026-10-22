// Package cmd wires the broker command line.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

const (
	groupHuman = "human"
	groupAgent = "agent"
)

// Execute runs broker with os.Args and returns the process exit status.
func Execute(version string) int {
	return run(version, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func run(version string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := newRoot(version)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		var status exitStatus
		if errors.As(err, &status) {
			return int(status)
		}
		return agenterr.Write(stderr, err)
	}
	return 0
}

// exitStatus is exec's child's exit status, which broker exits with
// silently: the child has already reported whatever went wrong.
type exitStatus int

func (s exitStatus) Error() string { return fmt.Sprintf("exit status %d", int(s)) }

func newRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "broker",
		Short: "Local secret store for AI agents",
		Long: "broker keeps secrets in age-encrypted files. Humans authorize with a\n" +
			"hardware touch; agents run commands with secrets without seeing them.",
		Version:           version,
		Args:              usageArgs(cobra.NoArgs),
		RunE:              func(c *cobra.Command, _ []string) error { return c.Help() },
		PersistentPreRunE: func(*cobra.Command, []string) error { return checkPlatform() },
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return agenterr.New(agenterr.Usage, "%s", err)
	})
	root.AddGroup(
		&cobra.Group{ID: groupHuman, Title: "Human commands:"},
		&cobra.Group{ID: groupAgent, Title: "Agent commands (never prompt):"},
	)

	root.AddCommand(
		newInitCmd(),
		newSetCmd(),
		newRmCmd(),
		newGrantCmd(),
		newRevokeCmd(),
		newGetCmd(),
		newImportCmd(),
		newBackupCmd(),
		newRestoreCmd(),
		newLogCmd(),
		newLsCmd(),
		newExecCmd(),
	)
	return root
}

// usageArgs reports positional argument errors as agenterr.Usage.
func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if err := validate(c, args); err != nil {
			return agenterr.New(agenterr.Usage, "%s", err)
		}
		return nil
	}
}

func openVault() (*vault.Vault, error) {
	p, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	return vault.Open(p)
}

// userKeyring opens the kernel user keyring. Tests that cannot isolate it
// replace this so they never touch the real one.
var userKeyring = keyring.User
