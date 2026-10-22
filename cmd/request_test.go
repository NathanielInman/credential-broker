package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/NathanielInman/credential-broker/internal/approval"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/keyring/keyringtest"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

const (
	// envWindow makes the test binary a fake request window that acts
	// out a script: steps separated by ";", run when the request arrives
	// ("approve", "deny", "for=SECS", "close", "wait") or answer prompts
	// ("value=TEXT"), and "retry" approves again after a failed unlock.
	envWindow = "BROKER_TEST_WINDOW"
	// envWindowLog is where the fake window appends every message it gets.
	envWindowLog = "BROKER_TEST_WINDOW_LOG"
	// envBeWindow marks the process that is the window.
	envBeWindow = "BROKER_TEST_BE_WINDOW"
)

func init() {
	if os.Getenv(envBeWindow) != "" {
		os.Exit(fakeWindow())
	}
	// An approver started by a test shows its window with the script.
	if os.Getenv(envBroker) != "" && os.Getenv(envWindow) != "" {
		dialogCommand = func() (*exec.Cmd, error) {
			exe, err := os.Executable()
			if err != nil {
				return nil, err
			}
			cmd := exec.Command(exe)
			cmd.Env = append(os.Environ(), envBeWindow+"=1")
			return cmd, nil
		}
	}
}

func fakeWindow() int {
	script := strings.Split(os.Getenv(envWindow), ";")
	logf, err := os.OpenFile(os.Getenv(envWindowLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return 1
	}
	defer logf.Close()
	enc := json.NewEncoder(os.Stdout)
	step := func(want string) (string, bool) {
		for _, s := range script {
			if k, v, _ := strings.Cut(s, "="); k == want {
				return v, true
			}
		}
		return "", false
	}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		logf.Write(append(sc.Bytes(), '\n'))
		var m approval.Message
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			return 1
		}
		switch m.Type {
		case "request":
			for _, s := range script {
				k, v, _ := strings.Cut(s, "=")
				switch k {
				case "approve", "deny":
					enc.Encode(approval.Reply{Type: k})
				case "for":
					n, _ := strconv.ParseInt(v, 10, 64)
					enc.Encode(approval.Reply{Type: "for", For: n})
				case "close":
					return 0
				}
			}
		case "prompt":
			v, _ := step("value")
			enc.Encode(approval.Reply{Type: "value", ID: m.Prompt.ID, Value: v})
		case "status":
			if _, ok := step("retry"); ok && m.Retry {
				enc.Encode(approval.Reply{Type: "approve"})
			}
		case "done":
			return 0
		}
	}
	return 0
}

// useWindow routes requests to a fake window acting out script, through
// a real approver process, and returns the window's message log.
func useWindow(t *testing.T, script string) string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "window.log")
	t.Setenv(envWindow, script)
	t.Setenv(envWindowLog, log)
	origWindow, origLaunch := requestWindow, launchApprover
	requestWindow = func() error { return nil }
	launchApprover = func(ctx context.Context, req []byte, stderr io.Writer) ([]byte, error) {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cmd := exec.CommandContext(ctx, exe, "approve")
		cmd.Env = append(os.Environ(), envBroker+"=1")
		cmd.Stdin = bytes.NewReader(req)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, stderr
		err = cmd.Run()
		return out.Bytes(), err
	}
	t.Cleanup(func() { requestWindow, launchApprover = origWindow, origLaunch })
	return log
}

// shown returns the request the fake window was shown, and every message.
func shown(t *testing.T, log string) (approval.Request, []approval.Message) {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var req approval.Request
	var msgs []approval.Message
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var m approval.Message
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("window log line %q: %v", line, err)
		}
		if m.Request != nil {
			req = *m.Request
		}
		msgs = append(msgs, m)
	}
	return req, msgs
}

func TestRequestApproved(t *testing.T) {
	grantVault(t)
	log := useWindow(t, "approve")
	out, _ := mustRun(t, "request", "API_TOKEN", "API_USER", "--for", "30m", "--reason", "deploy the dev box")
	if !strings.HasPrefix(out, "granted API_TOKEN for 30m (until ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("request output %q", out)
	}
	if d := time.Until(granted(t)["API_TOKEN"]); d < 29*time.Minute || d > 30*time.Minute {
		t.Fatalf("grant lasts %s", d)
	}

	req, msgs := shown(t, log)
	if len(req.Secrets) != 1 || req.Secrets[0].Name != "API_TOKEN" {
		t.Errorf("window showed secrets %+v; want only the private API_TOKEN", req.Secrets)
	}
	if req.Reason != "deploy the dev box" || req.For != 1800 || req.Zone != "plain" || req.Approve != "click" {
		t.Errorf("window showed %+v", req)
	}
	if want := []int64{300, 900, 1800}; !equalInts(req.Choices, want) {
		t.Errorf("choices %v, want %v", req.Choices, want)
	}
	if len(req.Ancestry) == 0 || req.Cwd == "" {
		t.Errorf("window showed no requester: ancestry %v cwd %q", req.Ancestry, req.Cwd)
	}
	if last := msgs[len(msgs)-1]; last.Type != "done" || last.Error || last.Text != "Granted for 30m" {
		t.Errorf("window ended with %+v", last)
	}

	entries := auditLog(t, "--name", "API_TOKEN")
	e := entries[len(entries)-1]
	if e.Action != audit.Request || !e.OK || e.Reason != "deploy the dev box" || e.For != "30m" || e.PID != os.Getpid() {
		t.Errorf("audit entry %+v", e)
	}
}

func TestRequestDenied(t *testing.T) {
	grantVault(t)
	useWindow(t, "deny")
	_, stderr, code := runStdin(t, "", "request", "API_TOKEN")
	if code != 77 || errorCode(t, stderr) != "denied" {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if g := granted(t); len(g) != 0 {
		t.Fatalf("granted %v after a deny", g)
	}
	entries := auditLog(t, "--name", "API_TOKEN")
	if e := entries[len(entries)-1]; e.Action != audit.Request || e.Error != "denied" {
		t.Errorf("audit entry %+v", e)
	}
}

func TestRequestClosedWindowDenies(t *testing.T) {
	grantVault(t)
	useWindow(t, "close")
	if _, stderr, code := runStdin(t, "", "request", "API_TOKEN"); code != 77 || errorCode(t, stderr) != "denied" {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestRequestShorterDuration(t *testing.T) {
	grantVault(t)
	useWindow(t, "for=900;approve")
	if out, _ := mustRun(t, "request", "API_TOKEN", "--for", "2h"); !strings.HasPrefix(out, "granted API_TOKEN for 15m ") {
		t.Fatalf("request output %q", out)
	}
	if d := time.Until(granted(t)["API_TOKEN"]); d > 15*time.Minute {
		t.Fatalf("grant lasts %s, want the 15m the human picked", d)
	}
}

func TestRequestTimeout(t *testing.T) {
	grantVault(t)
	log := useWindow(t, "wait")
	_, stderr, code := runStdin(t, "", "request", "API_TOKEN", "--timeout", "1s")
	if code != 75 || errorCode(t, stderr) != "request_timeout" || !strings.Contains(stderr, `"ask_human":"broker grant API_TOKEN --for 1h"`) {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, msgs := shown(t, log); msgs[len(msgs)-1].Text != "Timed out" {
		t.Errorf("window ended with %+v", msgs[len(msgs)-1])
	}
}

func TestRequestAlreadyGranted(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	launches := 0
	orig := launchApprover
	launchApprover = func(context.Context, []byte, io.Writer) ([]byte, error) {
		launches++
		return nil, nil
	}
	t.Cleanup(func() { launchApprover = orig })
	if out, _ := mustRun(t, "request", "API_TOKEN", "API_USER"); out != "" || launches != 0 {
		t.Fatalf("output %q, %d windows for names that need none", out, launches)
	}
}

func TestRequestUnknownName(t *testing.T) {
	grantVault(t)
	useWindow(t, "approve")
	if _, stderr, code := runStdin(t, "", "request", "NOPE"); errorCode(t, stderr) != "unknown_secret" || code != 78 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestRequestWithoutWindow(t *testing.T) {
	grantVault(t)
	orig := requestWindow
	requestWindow = func() error { return os.ErrNotExist }
	t.Cleanup(func() { requestWindow = orig })
	_, stderr, code := runStdin(t, "", "request", "API_TOKEN")
	if code != 77 || errorCode(t, stderr) != "no_grant" || !strings.Contains(stderr, `"ask_human":"broker grant API_TOKEN --for 1h"`) {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestExecRequest(t *testing.T) {
	grantVault(t)
	log := useWindow(t, "approve")
	out, stderr, code := runStdin(t, "", "exec", "--request", "--reason", "check the token",
		"API_TOKEN", "API_USER", "--", "sh", "-c", `echo "$API_USER $API_TOKEN"`)
	if code != 0 || out != "nate [REDACTED:API_TOKEN]\n" || !strings.Contains(stderr, "granted API_TOKEN for 1h") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	req, _ := shown(t, log)
	if strings.Join(req.Command, " ") != `sh -c echo "$API_USER $API_TOKEN"` || req.Reason != "check the token" {
		t.Errorf("window showed command %q reason %q", req.Command, req.Reason)
	}
}

func TestExecWithoutRequestStillFails(t *testing.T) {
	grantVault(t)
	useWindow(t, "approve")
	if _, stderr, code := runStdin(t, "", "exec", "API_TOKEN", "--", "true"); code != 77 || errorCode(t, stderr) != "no_grant" {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestApproveChecksTheRequester(t *testing.T) {
	grantVault(t)
	useWindow(t, "approve")
	// PID 1 is not broker, so the request is not shown, let alone granted.
	req, _ := json.Marshal(requestFile{Names: []string{"API_TOKEN"}, For: 60, PID: 1, Deadline: time.Now().Add(time.Minute)})
	out, err := launchApprover(context.Background(), req, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var res requestResult
	if err := json.Unmarshal(lastLine(out), &res); err != nil || res.Error == "" || !strings.Contains(res.Message, "not broker") {
		t.Fatalf("result %q, %v", out, err)
	}
	if g := granted(t); len(g) != 0 {
		t.Fatalf("granted %v", g)
	}
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// passphraseVault is grantVault's vault but unlocked by a passphrase file.
func passphraseVault(t *testing.T, pass string) {
	t.Helper()
	keyringtest.Require(t)
	home := t.TempDir()
	t.Setenv("BROKER_HOME", home)
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	r, err := age.NewScryptRecipient(pass)
	if err != nil {
		t.Fatal(err)
	}
	r.SetWorkFactor(10)
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, id.String()+"\n")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "passphrase-identity.age")
	if err := os.WriteFile(file, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	ui := vault.TerminalUI(io.Discard)
	ui.RequestValue = func(string, string, bool) (string, error) { return pass, nil }
	v, err := vault.Init(p, []string{file}, vault.InitOptions{UI: ui})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("API_TOKEN", []byte("token-value"), vault.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runStdin(t, "", "revoke", "--all") })
}

func TestRequestPassphrase(t *testing.T) {
	passphraseVault(t, "correct horse")
	log := useWindow(t, "value=correct horse")
	if out, _ := mustRun(t, "request", "API_TOKEN"); !strings.HasPrefix(out, "granted API_TOKEN for 1h ") {
		t.Fatalf("request output %q", out)
	}
	req, msgs := shown(t, log)
	if req.Zone != "passphrase" || req.Approve != "passphrase" {
		t.Errorf("zone %q approve %q", req.Zone, req.Approve)
	}
	var prompt *approval.Prompt
	for _, m := range msgs {
		if m.Prompt != nil {
			prompt = m.Prompt
		}
	}
	if prompt == nil || !prompt.Secret || !strings.Contains(prompt.Text, "passphrase-identity.age") {
		t.Errorf("prompt %+v", prompt)
	}
}

func TestRequestWrongPassphraseRetries(t *testing.T) {
	passphraseVault(t, "correct horse")
	log := useWindow(t, "value=wrong")
	_, stderr, code := runStdin(t, "", "request", "API_TOKEN")
	if code != 1 || !strings.Contains(stderr, "wrong passphrase") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	// Asked again right away, with the error showing, never a Try again.
	prompts, errs := 0, 0
	_, msgs := shown(t, log)
	for _, m := range msgs {
		if m.Prompt != nil {
			prompts++
		}
		if m.Type == "status" && m.Error {
			errs++
			if m.Retry || m.Text != "Wrong passphrase." {
				t.Errorf("status %+v", m)
			}
		}
	}
	if prompts != unlockAttempts || errs != unlockAttempts-1 {
		t.Errorf("%d prompts and %d errors, want %d and %d", prompts, errs, unlockAttempts, unlockAttempts-1)
	}
	if last := msgs[len(msgs)-1]; last.Type != "done" || !last.Error || last.Text != "Wrong passphrase." {
		t.Errorf("window ended with %+v", last)
	}
}

func TestApproveBy(t *testing.T) {
	hw := func(stub string) config.Unlock { return config.Unlock{Path: "/k", Identity: stub} }
	pass := config.Unlock{Path: "/p", Recipient: "age1p", Passphrase: true}
	plain := config.Unlock{Path: "/k", Recipient: "age1k"}
	for _, tc := range []struct {
		unlocks    []config.Unlock
		by, device string
	}{
		{[]config.Unlock{hw("AGE-PLUGIN-YUBIKEY-1QQ")}, "touch", "yubikey"},
		{[]config.Unlock{hw("AGE-PLUGIN-SE-1QQ"), pass}, "touch", "se"},
		{[]config.Unlock{pass}, "passphrase", ""},
		// A TPM may unlock with nobody there, so a click comes first.
		{[]config.Unlock{hw("AGE-PLUGIN-TPM-1QQ")}, "click", "tpm"},
		{[]config.Unlock{hw("AGE-PLUGIN-YUBIKEY-1QQ"), plain}, "click", ""},
	} {
		by, device := approveBy(&config.Config{Unlock: tc.unlocks})
		if by != tc.by || device != tc.device {
			t.Errorf("%+v: %q %q, want %q %q", tc.unlocks, by, device, tc.by, tc.device)
		}
	}
}
