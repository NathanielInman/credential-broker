package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/diceware"
	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// passphraseWords is the length of a generated passphrase: 7 diceware
// words, about 90 bits.
const passphraseWords = 7

// minPassphrase is the typed passphrase length under which backup warns. A
// backup can be guessed offline with no rate limit.
const minPassphrase = 20

// readPassword prompts on stderr and reads a line from the terminal without
// echo. Callers check requireTTY first. Tests replace it.
var readPassword = func(c *cobra.Command, prompt string) ([]byte, error) {
	fmt.Fprint(c.ErrOrStderr(), prompt)
	b, err := term.ReadPassword(int(c.InOrStdin().(*os.File).Fd()))
	fmt.Fprintln(c.ErrOrStderr())
	return b, err
}

func newBackupCmd() *cobra.Command {
	var output string
	var generate, includeLog bool
	c := &cobra.Command{
		Use:   "backup -o FILE",
		Short: "Write a passphrase-encrypted copy of the vault",
		Long: "backup writes the whole vault, with the private tier identity unwrapped\n" +
			"(one touch), to FILE encrypted with a passphrase alone. The passphrase is\n" +
			"all it takes to restore it, and all that protects it.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) (err error) {
			e := audit.Entry{Action: audit.Backup}
			defer func() { record(c, e, err) }()
			if output == "" {
				return agenterr.New(agenterr.Usage, "backup needs -o FILE")
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			ask := "broker backup -o " + output
			if generate {
				ask += " --generate-passphrase"
			}
			if includeLog {
				ask += " --include-log"
			}
			if err := requireTTY(c.InOrStdin(), ask); err != nil {
				return err
			}
			pass, err := backupPassphrase(c, generate)
			if err != nil {
				return err
			}
			id, err := v.UnlockPrivate(vault.TerminalUI(c.ErrOrStderr()))
			if err != nil {
				return err
			}
			var secrets []vault.Secret
			err = paths.WriteFileFunc(output, func(w io.Writer) error {
				var err error
				secrets, err = v.Backup(w, pass, id, vault.BackupOptions{IncludeLog: includeLog, Version: c.Root().Version})
				return err
			})
			if err != nil {
				return err
			}
			// Log every secret the backup carries, so log --name finds it.
			for _, s := range secrets {
				e.Names = append(e.Names, s.Name)
			}
			e.Tier = tierOf(v, e.Names)
			fmt.Fprintf(c.OutOrStdout(), "backed up %d secrets to %s\n", len(secrets), output)
			if generate {
				fmt.Fprintf(c.ErrOrStderr(), "passphrase: %s\n"+
					"write it down: nothing else can restore this backup\n", pass)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "backup file to write")
	c.Flags().BoolVar(&generate, "generate-passphrase", false, "generate a 7 word diceware passphrase")
	c.Flags().BoolVar(&includeLog, "include-log", false, "include the audit log")
	return c
}

// backupPassphrase generates a passphrase, or reads one typed twice and
// warns if it is short.
func backupPassphrase(c *cobra.Command, generate bool) (string, error) {
	if generate {
		return diceware.Passphrase(passphraseWords)
	}
	pass, err := readPassword(c, "backup passphrase: ")
	if err != nil {
		return "", err
	}
	if len(pass) == 0 {
		return "", agenterr.New(agenterr.Usage, "the passphrase is empty; type one or pass --generate-passphrase")
	}
	again, err := readPassword(c, "again: ")
	if err != nil {
		return "", err
	}
	if !bytes.Equal(pass, again) {
		return "", agenterr.New(agenterr.Usage, "the passphrases do not match")
	}
	if n := utf8.RuneCount(pass); n < minPassphrase {
		fmt.Fprintf(c.ErrOrStderr(), "broker: warning: the passphrase is only %d characters and a backup can be "+
			"guessed offline with no rate limit; consider --generate-passphrase\n", n)
	}
	return string(pass), nil
}

func newRestoreCmd() *cobra.Command {
	var identities []string
	var allowPlain, force bool
	c := &cobra.Command{
		Use:   "restore FILE",
		Short: "Rebuild the vault from a backup",
		Long: "restore rebuilds the vault from a backup and wraps the private tier to the\n" +
			"--identity files, or to the unlock identities in the backup (the same\n" +
			"YubiKey). The local audit log is kept; the backup's log, if any, becomes\n" +
			"the log only where there is none, and is saved beside it otherwise.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			file := args[0]
			e := audit.Entry{Action: audit.Restore}
			defer func() { record(c, e, err) }()
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			ask := "broker restore " + file
			for _, id := range identities {
				ask += " --identity " + id
			}
			if allowPlain {
				ask += " --insecure-plain-identity"
			}
			if force {
				ask += " --force"
			}
			res, err := vault.Restore(p, f, func() (string, error) {
				if err := requireTTY(c.InOrStdin(), ask); err != nil {
					return "", err
				}
				pass, err := readPassword(c, "backup passphrase: ")
				return string(pass), err
			}, vault.RestoreOptions{
				IdentityFiles: identities,
				AllowPlain:    allowPlain,
				Force:         force,
				UI:            vault.TerminalUI(c.ErrOrStderr()),
			})
			if res == nil {
				return err
			}
			// The vault is rewritten even if its log was not placed, so the
			// audit entry names what came back either way.
			secrets, lerr := res.Vault.List()
			for _, s := range secrets {
				e.Names = append(e.Names, s.Name)
			}
			e.Tier = tierOf(res.Vault, e.Names)
			if res.Replaced {
				defer revokeReplaced(c)
			}
			if err != nil {
				return err
			}
			if lerr != nil {
				return lerr
			}

			out := c.OutOrStdout()
			m := res.Manifest
			fmt.Fprintf(out, "restored %d secrets from %s (backed up %s", len(secrets), file,
				m.Created.Local().Format(time.DateTime))
			if m.Version != "" {
				fmt.Fprintf(out, " by broker %s", m.Version)
			}
			fmt.Fprintln(out, ")")
			printUnlocks(out, res.Vault)
			switch {
			case res.Log == p.LogFile():
				fmt.Fprintf(out, "the backup's audit log is now %s\n", res.Log)
			case res.Log != "":
				fmt.Fprintf(out, "kept the local audit log; the backup's is in %s\n", res.Log)
			}
			return nil
		},
	}
	c.Flags().StringArrayVar(&identities, "identity", nil, "wrap the private tier to this identity instead of the backup's (repeatable)")
	c.Flags().BoolVar(&allowPlain, "insecure-plain-identity", false, "allow plain AGE-SECRET-KEY identities (tests only: they protect nothing)")
	c.Flags().BoolVar(&force, "force", false, "replace an existing vault")
	return c
}

// revokeReplaced drops every grant after restore replaced a vault: they
// hold the old vault's values, which exec must not hand out under the
// restored names. Best effort, like rm: a grant left behind still expires.
func revokeReplaced(c *cobra.Command) {
	var revoked []string
	kr, err := userKeyring()
	if err == nil {
		var grants []keyring.Grant
		grants, err = kr.List()
		for _, g := range grants {
			if _, rerr := kr.Revoke(g.Name); rerr != nil {
				err = rerr
				continue
			}
			revoked = append(revoked, g.Name)
		}
	}
	if len(revoked) > 0 {
		slices.Sort(revoked)
		fmt.Fprintf(c.OutOrStdout(), "revoked grants from the replaced vault: %s\n", strings.Join(revoked, ", "))
	}
	if err != nil {
		fmt.Fprintf(c.ErrOrStderr(), "broker: warning: grants from the replaced vault were not revoked (they still expire): %v\n", err)
	}
}
