package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/approval"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/procinfo"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// requestTimeout is how long a request waits for the human by default.
const requestTimeout = 2 * time.Minute

// requestGrace is how long past its deadline request waits for the
// approver to report back before giving up on it.
const requestGrace = 15 * time.Second

// unlockAttempts bounds how often the window offers to retry a failed
// unlock (a mistyped passphrase, an unplugged key).
const unlockAttempts = 3

// requestChoices are the shorter grant durations the window offers.
var requestChoices = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour}

// approverEnv is the environment the approver gets: enough to find the
// vault, the desktop, and age plugins, and nothing else of the agent's.
var approverEnv = []string{
	"HOME", "PATH", "LANG", "LC_ALL", "USER", "TMPDIR",
	"BROKER_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR",
	"WAYLAND_DISPLAY", "DISPLAY", "XAUTHORITY", "DBUS_SESSION_BUS_ADDRESS", "GDK_BACKEND",
}

// requestFile is what request hands the approver on stdin.
type requestFile struct {
	Names    []string  `json:"names"`
	For      int64     `json:"for"`
	Reason   string    `json:"reason,omitempty"`
	Command  []string  `json:"command,omitempty"`
	PID      int       `json:"pid"`
	Deadline time.Time `json:"deadline"`
}

// requestResult is the approver's answer on stdout.
type requestResult struct {
	// Error is an agenterr code; empty means granted.
	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
	Granted []grantedSecret `json:"granted,omitempty"`
}

type grantedSecret struct {
	Name    string    `json:"name"`
	For     string    `json:"for"`
	Expires time.Time `json:"expires"`
}

func newRequestCmd() *cobra.Command {
	var dur, timeout time.Duration
	var reason string
	c := &cobra.Command{
		Use:   "request NAME... [--for DUR] [--reason TEXT]",
		Short: "Ask the human for a grant in a window on their desktop",
		Long: "request opens a window on the human's desktop asking them to grant the\n" +
			"private NAMEs, and waits (up to --timeout) for them to approve with a\n" +
			"touch, their passphrase, or a click, depending on the vault. --reason is\n" +
			"shown to them as your words. Names already granted need nothing.",
		GroupID: groupAgent,
		Args:    usageArgs(cobra.MinimumNArgs(1)),
		RunE: func(c *cobra.Command, names []string) error {
			v, err := openVault()
			if err != nil {
				record(c, audit.Entry{Action: audit.Request, Names: names}, err)
				return err
			}
			ttl, err := grantDuration(v.Config(), c.Flags().Changed("for"), dur, false)
			if err != nil {
				return err
			}
			return requestGrants(c, v, names, ttl, reason, nil, timeout, c.OutOrStdout())
		},
	}
	c.Flags().DurationVar(&dur, "for", 0, "grant duration to ask for (default from config, max 24h)")
	c.Flags().StringVar(&reason, "reason", "", "why you need them, shown to the human")
	c.Flags().DurationVar(&timeout, "timeout", requestTimeout, "how long to wait for the human")
	return c
}

// requestGrants asks the human, in the request window, to grant whichever
// private names have no live grant, and reports each grant on out. When
// there is no window to ask in, it fails with the same no_grant error exec
// gives, so the agent can still hand the human a terminal command.
func requestGrants(c *cobra.Command, v *vault.Vault, names []string, ttl time.Duration, reason string, argv []string, timeout time.Duration, out io.Writer) error {
	missing, err := ungranted(v, names)
	if err != nil || len(missing) == 0 {
		return err
	}
	var missingNames []string
	for _, s := range missing {
		missingNames = append(missingNames, s.Name)
	}
	e := audit.Entry{Action: audit.Request, Names: missingNames, Tier: string(vault.Private), For: shortDuration(ttl), Reason: reason}
	if len(argv) > 0 {
		e.Cmd = argv[0]
	}
	noWindow := func(why error) error {
		err := &agenterr.Error{
			Code:     agenterr.NoGrant,
			Names:    missingNames,
			AskHuman: "broker grant " + strings.Join(missingNames, " ") + " --for " + shortDuration(ttl),
			Message:  "no request window: " + why.Error(),
		}
		record(c, e, err)
		return err
	}
	if timeout <= 0 {
		return agenterr.New(agenterr.Usage, "--timeout %s is not positive", timeout)
	}
	if err := requestWindow(); err != nil {
		return noWindow(err)
	}

	req, err := json.Marshal(requestFile{
		Names:    missingNames,
		For:      int64(ttl / time.Second),
		Reason:   reason,
		Command:  argv,
		PID:      os.Getpid(),
		Deadline: time.Now().Add(timeout).UTC(),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+requestGrace)
	defer cancel()
	stdout, lerr := launchApprover(ctx, req, c.ErrOrStderr())
	var res requestResult
	if err := json.Unmarshal(lastLine(stdout), &res); err != nil {
		if ctx.Err() != nil {
			return requestTimedOut(missingNames, ttl)
		}
		if lerr == nil {
			lerr = err
		}
		return noWindow(fmt.Errorf("the approver failed: %w", lerr))
	}
	switch agenterr.Code(res.Error) {
	case "":
	case agenterr.RequestTimeout:
		return requestTimedOut(missingNames, ttl)
	default:
		return &agenterr.Error{Code: agenterr.Code(res.Error), Names: missingNames, Message: res.Message}
	}
	for _, g := range res.Granted {
		fmt.Fprintf(out, "granted %s for %s (until %s)\n", g.Name, g.For, g.Expires.Local().Format(time.DateTime))
	}
	return nil
}

func requestTimedOut(names []string, ttl time.Duration) error {
	return &agenterr.Error{
		Code:     agenterr.RequestTimeout,
		Names:    names,
		AskHuman: "broker grant " + strings.Join(names, " ") + " --for " + shortDuration(ttl),
		Message:  "nobody answered the request window in time",
	}
}

// lastLine is the last non-empty line of b, where the approver's result is.
func lastLine(b []byte) []byte {
	b = bytes.TrimRight(b, "\n")
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return b
}

// ungranted resolves names to the private secrets among them without a
// live grant. Unknown names fail, since no grant would fix them.
func ungranted(v *vault.Vault, names []string) ([]vault.Secret, error) {
	var out []vault.Secret
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
		if s.Tier == vault.Private {
			out = append(out, s)
		}
	}
	if len(unknown) > 0 {
		return nil, &agenterr.Error{Code: agenterr.UnknownSecret, Names: unknown}
	}
	if len(out) == 0 {
		return nil, nil
	}
	kr, err := userKeyring()
	if err != nil {
		return nil, err
	}
	var missing []vault.Secret
	for _, s := range out {
		live, _, err := lookupGrant(kr, s.Name)
		if err != nil {
			return nil, err
		}
		if !live {
			missing = append(missing, s)
		}
	}
	return missing, nil
}

// requestWindow reports why no request window can open, or nil.
var requestWindow = func() error {
	if err := desktopSession(); err != nil {
		return err
	}
	_, err := dialogPath()
	return err
}

// dialogPath finds broker-dialog beside the broker executable, else on
// PATH.
func dialogPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "broker-dialog")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	p, err := exec.LookPath("broker-dialog")
	if err != nil {
		return "", errors.New("broker-dialog is not installed beside broker or on PATH")
	}
	return p, nil
}

// dialogCommand is the request window program. Tests replace it.
var dialogCommand = func() (*exec.Cmd, error) {
	p, err := dialogPath()
	if err != nil {
		return nil, err
	}
	return exec.Command(p), nil
}

// launchApprover runs "broker approve" with req on its stdin and returns
// its stdout. The approver unlocks the vault, so it must not be the
// agent's descendant: under ptrace_scope 1 a process may read its
// descendants' memory. systemd-run makes it a child of the user's service
// manager instead, which also keeps the window up if the agent's command
// is killed. Without systemd it runs in a session of its own. Tests
// replace this.
var launchApprover = func(ctx context.Context, req []byte, stderr io.Writer) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	var env []string
	for _, k := range approverEnv {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	var cmd *exec.Cmd
	if run, err := exec.LookPath("systemd-run"); err == nil {
		var deadline requestFile
		_ = json.Unmarshal(req, &deadline)
		max := int(time.Until(deadline.Deadline)/time.Second) + int(requestGrace/time.Second)
		args := []string{"--user", "--quiet", "--collect", "--wait", "--pipe",
			fmt.Sprintf("--property=RuntimeMaxSec=%d", max)}
		for _, kv := range env {
			args = append(args, "--setenv="+kv)
		}
		cmd = exec.CommandContext(ctx, run, append(args, "--", exe, "approve")...)
	} else {
		cmd = exec.CommandContext(ctx, exe, "approve")
		cmd.Env = env
		cmd.SysProcAttr = newSession()
	}
	cmd.Stdin = bytes.NewReader(req)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, stderr
	err = cmd.Run()
	return out.Bytes(), err
}

func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "approve",
		Short:  "Show a request window and grant what the human approves (run by request)",
		Hidden: true,
		Args:   usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			// Nothing running as this user may attach to the process that
			// holds the unwrapped private tier identity.
			if err := harden(); err != nil {
				return err
			}
			var req requestFile
			if err := json.NewDecoder(io.LimitReader(c.InOrStdin(), 1<<20)).Decode(&req); err != nil {
				return agenterr.New(agenterr.Usage, "approve reads a request on stdin: %v", err)
			}
			e := audit.Entry{Action: audit.Request, Names: req.Names, Tier: string(vault.Private), Reason: req.Reason, PID: req.PID}
			if len(req.Command) > 0 {
				e.Cmd = req.Command[0]
			}
			res, err := approve(&req, &e)
			if err != nil {
				ae := agenterr.From(err)
				res = requestResult{Error: string(ae.Code), Message: ae.Message}
			}
			if e.PID != 0 {
				record(c, e, err)
			}
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetEscapeHTML(false)
			return enc.Encode(res)
		},
	}
}

// approve shows req in the request window and grants what the human
// approves. It fills in e for the audit log as it learns more.
func approve(req *requestFile, e *audit.Entry) (requestResult, error) {
	var res requestResult
	ttl := time.Duration(req.For) * time.Second
	if ttl <= 0 || ttl > maxGrant {
		return res, agenterr.New(agenterr.Usage, "grant duration %s is not between 0 and %s", ttl, shortDuration(maxGrant))
	}
	e.For = shortDuration(ttl)
	cwd, ancestry, err := requester(req.PID)
	if err != nil {
		e.PID = 0 // not a requester worth logging as one
		return res, err
	}
	e.Cwd, e.Ancestry = cwd, ancestry

	p, err := paths.Resolve()
	if err != nil {
		return res, err
	}
	v, err := vault.Open(p)
	if err != nil {
		return res, err
	}
	// One window at a time. A request that waited behind another often
	// finds its names granted by then and needs no window at all.
	unlock, err := lockRequests(p, req.Deadline)
	if err != nil {
		return res, requestErr(err)
	}
	defer unlock()
	secrets, err := ungranted(v, req.Names)
	if err != nil || len(secrets) == 0 {
		return res, err
	}
	e.Names = nil
	for _, s := range secrets {
		e.Names = append(e.Names, s.Name)
	}

	dialog, err := dialogCommand()
	if err != nil {
		return res, err
	}
	home, _ := os.UserHomeDir()
	shown := approval.Request{
		For:      req.For,
		Choices:  durationChoices(ttl),
		Deadline: req.Deadline,
		Ancestry: ancestry,
		Cwd:      cwd,
		Home:     home,
		Command:  req.Command,
		Zone:     string(v.Config().Zone()),
		Reason:   req.Reason,
	}
	shown.Approve, shown.Device = approveBy(v.Config())
	for _, s := range secrets {
		shown.Secrets = append(shown.Secrets, approval.Secret{Name: s.Name, Description: s.Description})
	}
	sess, err := approval.Start(dialog, shown)
	if err != nil {
		return res, err
	}

	id, err := unlockApproved(sess, v, shown.Approve)
	if err != nil {
		sess.Finish(finishText(err), true)
		return res, requestErr(err)
	}
	ttl = sess.For()
	e.For = shortDuration(ttl)
	kr, err := userKeyring()
	if err != nil {
		sess.Finish("Could not grant: "+err.Error(), true)
		return res, err
	}
	for _, s := range secrets {
		value, err := v.Decrypt(s.Name, id)
		if err != nil {
			sess.Finish("Could not grant: "+err.Error(), true)
			return res, err
		}
		expires, err := kr.Add(s.Name, value, ttl)
		clear(value)
		if err != nil {
			sess.Finish("Could not grant: "+err.Error(), true)
			return res, err
		}
		res.Granted = append(res.Granted, grantedSecret{Name: s.Name, For: shortDuration(ttl), Expires: expires.UTC()})
	}
	sess.Finish("Granted for "+shortDuration(ttl), false)
	return res, nil
}

// unlockApproved waits for what approving takes and returns the private
// tier identity. With hardware the touch is the approval and unlocking
// starts at once, as it does with a passphrase, whose prompt appears in
// the window; a plain vault unlocks with no human at all, so it waits for
// Approve first. A failed unlock can be retried from the window.
func unlockApproved(sess *approval.Session, v *vault.Vault, by string) (age.Identity, error) {
	if by == "click" {
		if err := sess.WaitApprove(); err != nil {
			return nil, err
		}
	}
	for attempt := 1; ; attempt++ {
		type unlocked struct {
			id  age.Identity
			err error
		}
		ch := make(chan unlocked, 1)
		go func() {
			id, err := v.UnlockPrivate(sess.UI())
			ch <- unlocked{id, err}
		}()
		var u unlocked
		select {
		case u = <-ch:
		case <-sess.Done():
			// A plugin may still be waiting on a touch; the approver exits
			// right after, which ends it.
			return nil, sess.Err()
		}
		if u.err == nil {
			return u.id, nil
		}
		if attempt == unlockAttempts {
			return nil, u.err
		}
		// A mistyped passphrase is simply asked for again, with the error
		// under the field; anything else (an unplugged key) waits for the
		// human to fix it and press Try again.
		if wrongPassphrase(u.err) {
			sess.Status(unlockFailure(u.err), true, false)
			continue
		}
		sess.Status(unlockFailure(u.err)+" Approve to try again.", true, true)
		if err := sess.WaitApprove(); err != nil {
			return nil, err
		}
	}
}

// presencePlugins are the age plugins whose unlock itself needs the human
// there: a touch or a fingerprint. Another hardware plugin may unlock with
// nobody present (a TPM without a PIN), so approving starts with a click.
var presencePlugins = map[string]bool{"yubikey": true, "se": true, "fido2-hmac": true}

// approveBy is what approving a request takes for cfg's vault, and the
// plugin that asks for it: a touch or fingerprint when such a plugin is
// tried first, else the passphrase, else a click, for a vault that would
// otherwise unlock with no human at all.
func approveBy(cfg *config.Config) (by, device string) {
	if cfg.Zone() == config.ZonePlain {
		return "click", ""
	}
	for _, u := range cfg.Unlock {
		if u.Zone() != config.ZoneHardware {
			continue
		}
		if !presencePlugins[u.Plugin()] {
			return "click", u.Plugin()
		}
		return "touch", u.Plugin()
	}
	return "passphrase", ""
}

// durationChoices is ttl and the shorter standard durations.
func durationChoices(ttl time.Duration) []int64 {
	out := []int64{int64(ttl / time.Second)}
	for _, d := range requestChoices {
		if d < ttl {
			out = append(out, int64(d/time.Second))
		}
	}
	slices.Sort(out)
	return out
}

func wrongPassphrase(err error) bool { return strings.Contains(err.Error(), "wrong passphrase") }

func unlockFailure(err error) string {
	if wrongPassphrase(err) {
		return "Wrong passphrase."
	}
	return "Could not unlock: " + err.Error() + "."
}

func finishText(err error) string {
	switch {
	case errors.Is(err, approval.ErrDenied):
		return "Denied"
	case errors.Is(err, approval.ErrTimeout):
		return "Timed out"
	}
	return unlockFailure(err)
}

// requestErr maps a session's end to the agent's error code.
func requestErr(err error) error {
	switch {
	case errors.Is(err, approval.ErrDenied):
		return agenterr.New(agenterr.Denied, "the human denied the request; do not ask again unless they say so")
	case errors.Is(err, approval.ErrTimeout):
		return agenterr.New(agenterr.RequestTimeout, "nobody answered the request window in time")
	}
	return err
}

// requester checks that pid is a broker process and reads its working
// directory and process chain, so the window shows where the request
// really came from rather than what the request file claims.
func requester(pid int) (cwd string, ancestry []string, err error) {
	self, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	exe, err := procinfo.Exe(pid)
	if err != nil || !sameFile(exe, self) {
		return "", nil, fmt.Errorf("requesting process %d is gone or is not broker", pid)
	}
	cwd, _ = procinfo.Cwd(pid)
	return cwd, audit.Ancestry(pid), nil
}

func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}
