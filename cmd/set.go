package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func newSetCmd() *cobra.Command {
	var public, private, fromStdin bool
	var desc string
	c := &cobra.Command{
		Use:     "set NAME",
		Short:   "Store a secret from a hidden prompt or stdin",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			name := args[0]
			e := audit.Entry{Action: audit.Set, Names: []string{name}}
			defer func() { record(c, e, err) }()
			if public && private {
				return agenterr.New(agenterr.Usage, "--public and --private are exclusive")
			}
			if err := vault.ValidateName(name); err != nil {
				return err
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			value, err := readValue(c, name, fromStdin)
			if err != nil {
				return err
			}

			var opts vault.SetOptions
			switch {
			case public:
				opts.Tier = vault.Public
			case private:
				opts.Tier = vault.Private
			}
			if c.Flags().Changed("desc") {
				opts.Description = &desc
			}
			s, err := v.Set(name, value, opts)
			if err != nil {
				return err
			}
			e.Tier = string(s.Tier)
			fmt.Fprintf(c.OutOrStdout(), "stored %s in the %s tier\n", s.Name, s.Tier)
			return nil
		},
	}
	c.Flags().BoolVar(&public, "public", false, "store in the public tier (no touch, not redacted)")
	c.Flags().BoolVar(&private, "private", false, "move an existing public secret to the private tier")
	c.Flags().StringVar(&desc, "desc", "", "one-line description of what the secret is for")
	c.Flags().BoolVar(&fromStdin, "stdin", false, "read the value from stdin instead of a prompt")
	return c
}

// readValue reads the value from stdin (dropping one trailing newline) or
// from a hidden prompt on the terminal.
func readValue(c *cobra.Command, name string, fromStdin bool) ([]byte, error) {
	in := c.InOrStdin()
	if fromStdin {
		// Read a little past MaxValue so Set reports the overflow.
		value, err := io.ReadAll(io.LimitReader(in, vault.MaxValue+3))
		if err != nil {
			return nil, err
		}
		value = bytes.TrimSuffix(value, []byte("\n"))
		return bytes.TrimSuffix(value, []byte("\r")), nil
	}
	if err := agenterr.RequireTTY(in, "broker set "+name); err != nil {
		return nil, err
	}
	fmt.Fprintf(c.ErrOrStderr(), "value for %s: ", name)
	value, err := term.ReadPassword(int(in.(*os.File).Fd()))
	fmt.Fprintln(c.ErrOrStderr())
	return value, err
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Delete a secret",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			e := audit.Entry{Action: audit.Rm, Names: args}
			defer func() { record(c, e, err) }()
			v, err := openVault()
			if err != nil {
				return err
			}
			e.Tier = tierOf(v, args)
			if err := v.Remove(args[0]); err != nil {
				return err
			}
			// Drop any grant too, so exec cannot use a removed secret. Best
			// effort: a grant left behind still expires on its own.
			if kr, err := userKeyring(); err == nil {
				_, _ = kr.Revoke(args[0])
			}
			fmt.Fprintf(c.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
}
