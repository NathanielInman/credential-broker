package cmd

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/proc"
	"github.com/NathanielInman/credential-broker/internal/redact"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func newExecCmd() *cobra.Command {
	var request bool
	var reason string
	c := &cobra.Command{
		Use:   "exec NAME... -- CMD [ARGS...]",
		Short: "Run CMD with secrets in its environment, redacting its output",
		Long: "exec runs CMD with each NAME added to its environment. Public secrets\n" +
			"come from disk; private ones need a live grant. When stdout or stderr is\n" +
			"not a terminal, private values in it are replaced with [REDACTED:NAME].\n" +
			"stdin, signals, and the exit status pass through.\n\n" +
			"With --request, missing grants are asked for in a window on the human's\n" +
			"desktop (see broker request), showing CMD, and CMD runs once granted.",
		GroupID: groupAgent,
		Args:    execArgs,
		RunE: func(c *cobra.Command, args []string) error {
			dash := c.ArgsLenAtDash()
			names, argv := args[:dash], args[dash:]
			// The entry goes in before the child runs, which may take hours:
			// ok means its secrets were handed out. The child's own exit
			// status is not logged.
			e := audit.Entry{Action: audit.Exec, Names: names, Cmd: argv[0]}
			v, err := openVault()
			if err != nil {
				record(c, e, err)
				return err
			}
			e.Tier = tierOf(v, names)
			secrets, err := resolve(v, names)
			if request && agenterr.From(err).Code == agenterr.NoGrant {
				var ttl time.Duration
				if ttl, err = grantDuration(v.Config(), false, 0, false); err == nil {
					// Grant lines go to stderr: stdout is the child's.
					err = requestGrants(c, v, names, ttl, reason, argv, requestTimeout, c.ErrOrStderr())
				}
				if err == nil {
					secrets, err = resolve(v, names)
				}
			}
			record(c, e, err)
			if err != nil {
				return err
			}
			return runChild(c, argv, secrets)
		},
	}
	c.Flags().BoolVar(&request, "request", false, "ask the human for missing grants in a desktop window, then run")
	c.Flags().StringVar(&reason, "reason", "", "with --request, why you need them, shown to the human")
	return c
}

func execArgs(c *cobra.Command, args []string) error {
	dash := c.ArgsLenAtDash()
	if dash < 1 || dash == len(args) {
		return agenterr.New(agenterr.Usage, "usage: broker exec NAME... -- CMD [ARGS...]")
	}
	return nil
}

// resolved is a secret exec injects, with its value. Clear value after use.
type resolved struct {
	vault.Secret
	value []byte
}

// resolve reads each named secret's value: public from disk, private from
// its grant, never with a prompt. It fails before anything runs: first on
// unknown names, which no grant would fix, then on private names without
// a live grant.
func resolve(v *vault.Vault, names []string) ([]resolved, error) {
	var out []resolved
	var unknown []string
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		s, err := v.Get(name)
		if err != nil {
			if agenterr.From(err).Code == agenterr.UnknownSecret {
				unknown = append(unknown, name)
				continue
			}
			return nil, err
		}
		out = append(out, resolved{Secret: s})
	}
	if len(unknown) > 0 {
		return nil, &agenterr.Error{Code: agenterr.UnknownSecret, Names: unknown}
	}

	var kr *keyring.Keyring
	var ungranted []string
	for i := range out {
		r := &out[i]
		var err error
		if r.Tier == vault.Public {
			r.value, err = v.Decrypt(r.Name, nil)
		} else {
			if kr == nil {
				kr, err = userKeyring()
			}
			if err == nil {
				r.value, err = kr.Read(r.Name)
			}
			if errors.Is(err, keyring.ErrNotGranted) || errors.Is(err, keyring.ErrExpired) {
				ungranted = append(ungranted, r.Name)
				continue
			}
		}
		if err != nil {
			clearValues(out)
			return nil, err
		}
	}
	if len(ungranted) > 0 {
		clearValues(out)
		ask := "broker grant " + strings.Join(ungranted, " ")
		if ttl, err := grantDuration(v.Config(), false, 0, false); err == nil {
			ask += " --for " + shortDuration(ttl)
		}
		return nil, &agenterr.Error{Code: agenterr.NoGrant, Names: ungranted, AskHuman: ask}
	}
	return out, nil
}

func clearValues(rs []resolved) {
	for _, r := range rs {
		clear(r.value)
	}
}

// runChild runs argv with the secrets added to its environment, redacting
// private values from whichever of stdout and stderr is not a terminal,
// and returns the child's exit status as an exitStatus.
func runChild(c *cobra.Command, argv []string, secrets []resolved) error {
	// Values go only into the child's environment: never argv, never disk.
	env := os.Environ()
	var private []redact.Secret
	for _, s := range secrets {
		env = append(env, s.Name+"="+string(s.value))
		if s.Tier == vault.Private {
			private = append(private, redact.Secret{Name: s.Name, Value: s.value})
		}
	}
	child := exec.Command(argv[0], argv[1:]...)
	child.Env = env
	child.Stdin = c.InOrStdin()
	stdout := redactUnlessTTY(c.OutOrStdout(), private)
	stderr := redactUnlessTTY(c.ErrOrStderr(), private)
	child.Stdout, child.Stderr = stdout, stderr
	// The redactors hold their own copies.
	clearValues(secrets)

	status, err := proc.Run(child)
	// Wait has drained the pipes, so the held-back tails are final.
	for _, w := range []io.Writer{stdout, stderr} {
		if r, ok := w.(*redact.Writer); ok {
			if cerr := r.Close(); err == nil {
				err = cerr
			}
		}
	}
	var start *proc.StartError
	switch {
	case errors.As(err, &start):
		code := agenterr.CannotExec
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			code = agenterr.CommandNotFound
		}
		return agenterr.New(code, "%v", start.Err)
	case status != 0:
		return exitStatus(status)
	}
	return err
}

// redactUnlessTTY wraps w in a redactor unless w is a terminal, where a
// human is watching and interactive tools need the real thing.
func redactUnlessTTY(w io.Writer, secrets []redact.Secret) io.Writer {
	if f, ok := w.(*os.File); (ok && term.IsTerminal(int(f.Fd()))) || len(secrets) == 0 {
		return w
	}
	return redact.NewWriter(w, secrets)
}
