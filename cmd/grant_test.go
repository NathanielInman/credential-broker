package cmd

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/keyring/keyringtest"
)

// envBroker makes the test binary run broker itself, as a separate process.
const envBroker = "BROKER_TEST_RUN_BROKER"

func TestMain(m *testing.M) {
	if os.Getenv(envBroker) != "" {
		os.Exit(Execute("test"))
	}
	if !keyringtest.Isolated() {
		// keyringtest.Main reruns the tests isolated, or runs them right
		// here if it cannot. Either way this copy keeps off the real keyring.
		userKeyring = func() (*keyring.Keyring, error) {
			return nil, errors.New("user keyring is not isolated in tests")
		}
	}
	os.Exit(keyringtest.Main(m))
}

// grantVault is seedVault with an isolated keyring, revoking every grant
// afterwards.
func grantVault(t *testing.T) {
	t.Helper()
	keyringtest.Require(t)
	seedVault(t)
	t.Cleanup(func() { runStdin(t, "", "revoke", "--all") })
}

// seedVault is initVault with private API_TOKEN and DB_PASS and public
// API_USER, and a TTY check that always passes, since the plain test
// identity never prompts.
func seedVault(t *testing.T) {
	t.Helper()
	initVault(t)
	requireTTY = func(io.Reader, string) error { return nil }
	t.Cleanup(func() { requireTTY = defaultRequireTTY })
	for _, s := range []struct{ name, value, flag string }{
		{"API_TOKEN", "token-value", "--private"},
		{"DB_PASS", "db-password", "--private"},
		{"API_USER", "nate", "--public"},
	} {
		if _, stderr, code := runStdin(t, s.value, "set", s.name, "--stdin", s.flag); code != 0 {
			t.Fatalf("set %s exit %d: %s", s.name, code, stderr)
		}
	}
}

var defaultRequireTTY = requireTTY

func mustRun(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, code := runStdin(t, "", args...)
	if code != 0 {
		t.Fatalf("%v exit %d: %s", args, code, stderr)
	}
	return stdout, stderr
}

// granted returns ls's view of which secrets are granted, and expiries.
func granted(t *testing.T) map[string]time.Time {
	t.Helper()
	out := map[string]time.Time{}
	for _, e := range ls(t) {
		if !e.Granted {
			if e.Expires != nil {
				t.Errorf("%s not granted but expires %s", e.Name, *e.Expires)
			}
			continue
		}
		if e.Expires == nil {
			t.Fatalf("%s granted without expiry", e.Name)
		}
		exp, err := time.Parse(time.RFC3339, *e.Expires)
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name] = exp
	}
	return out
}

func TestGrantLsRevoke(t *testing.T) {
	grantVault(t)
	out, _ := mustRun(t, "grant", "API_TOKEN", "--for", "2h")
	if !strings.HasPrefix(out, "granted API_TOKEN for 2h (until ") {
		t.Fatalf("grant output %q", out)
	}

	g := granted(t)
	if d := time.Until(g["API_TOKEN"]); len(g) != 1 || d < 2*time.Hour-5*time.Second || d > 2*time.Hour+time.Second {
		t.Fatalf("granted = %v", g)
	}
	if out, _ := mustRun(t, "ls"); !regexp.MustCompile(`\nAPI_TOKEN +private +(2h|1h59m\d+s) `).MatchString(out) {
		t.Fatalf("human ls:\n%s", out)
	}
	kr, err := userKeyring()
	if err != nil {
		t.Fatal(err)
	}
	if v, err := kr.Read("API_TOKEN"); err != nil || string(v) != "token-value" {
		t.Fatalf("keyring value %q, %v", v, err)
	}

	if out, _ := mustRun(t, "revoke", "API_TOKEN", "DB_PASS"); out != "revoked API_TOKEN\nDB_PASS was not granted\n" {
		t.Fatalf("revoke output %q", out)
	}
	if g := granted(t); len(g) != 0 {
		t.Fatalf("granted after revoke = %v", g)
	}
	if _, err := kr.Read("API_TOKEN"); !errors.Is(err, keyring.ErrNotGranted) {
		t.Fatalf("Read after revoke: %v", err)
	}
}

func TestGrantDefaultDuration(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	if d := time.Until(granted(t)["API_TOKEN"]); d < time.Hour-5*time.Second || d > time.Hour+time.Second {
		t.Fatalf("default grant lasts %s, want 1h", d)
	}
}

func TestGrantExpires(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN", "--for", "1s")
	if g := granted(t); len(g) != 1 {
		t.Fatalf("granted = %v", g)
	}
	time.Sleep(2100 * time.Millisecond)
	if g := granted(t); len(g) != 0 {
		t.Fatalf("granted after expiry = %v", g)
	}
}

func TestGrantAllRevokeAll(t *testing.T) {
	grantVault(t)
	out, _ := mustRun(t, "grant", "--all", "--for", "10m")
	if strings.Count(out, "granted ") != 2 || strings.Contains(out, "API_USER") {
		t.Fatalf("grant --all output %q", out)
	}
	if g := granted(t); len(g) != 2 {
		t.Fatalf("granted = %v", g)
	}
	out, _ = mustRun(t, "revoke", "--all")
	if out != "revoked API_TOKEN\nrevoked DB_PASS\n" && out != "revoked DB_PASS\nrevoked API_TOKEN\n" {
		t.Fatalf("revoke --all output %q", out)
	}
	if out, _ := mustRun(t, "revoke", "--all"); out != "no grants to revoke\n" {
		t.Fatalf("second revoke --all output %q", out)
	}
}

func TestGrantPublicIsNoop(t *testing.T) {
	grantVault(t)
	out, stderr := mustRun(t, "grant", "API_USER")
	if out != "" || !strings.Contains(stderr, "API_USER is public") {
		t.Fatalf("stdout %q stderr %q", out, stderr)
	}
	if g := granted(t); len(g) != 0 {
		t.Fatalf("granted = %v", g)
	}
}

func TestRmDropsGrant(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	mustRun(t, "rm", "API_TOKEN")
	kr, err := userKeyring()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kr.Read("API_TOKEN"); !errors.Is(err, keyring.ErrNotGranted) {
		t.Fatalf("Read after rm: %v", err)
	}
}

func TestGrantErrors(t *testing.T) {
	grantVault(t)
	for _, tc := range []struct {
		args  []string
		code  string
		names string
	}{
		{[]string{"grant"}, "usage", ""},
		{[]string{"grant", "API_TOKEN", "--all"}, "usage", ""},
		{[]string{"grant", "lower"}, "usage", ""},
		{[]string{"grant", "API_TOKEN", "--for", "0s"}, "usage", ""},
		{[]string{"grant", "API_TOKEN", "--for", "25h"}, "usage", ""},
		{[]string{"grant", "NOPE", "API_TOKEN", "GONE"}, "unknown_secret", `"names":["NOPE","GONE"]`},
		{[]string{"revoke"}, "usage", ""},
		{[]string{"revoke", "X", "--all"}, "usage", ""},
		{[]string{"revoke", "lower"}, "usage", ""},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			_, stderr, _ := runStdin(t, "", tc.args...)
			if errorCode(t, stderr) != tc.code || !strings.Contains(stderr, tc.names) {
				t.Fatalf("stderr %s, want %s %s", stderr, tc.code, tc.names)
			}
		})
	}
	if g := granted(t); len(g) != 0 {
		t.Fatalf("failed grants granted %v", g)
	}

	mustRun(t, "grant", "API_TOKEN", "--for", "25h", "--force")
	if d := time.Until(granted(t)["API_TOKEN"]); d < 24*time.Hour {
		t.Fatalf("forced grant lasts %s", d)
	}
}

func TestGrantWithoutTTYNeedsHuman(t *testing.T) {
	grantVault(t)
	requireTTY = defaultRequireTTY
	_, stderr, code := runStdin(t, "", "grant", "API_TOKEN", "DB_PASS")
	if errorCode(t, stderr) != "needs_human" || code != 75 ||
		!strings.Contains(stderr, `"ask_human":"broker grant API_TOKEN DB_PASS --for 1h"`) {
		t.Fatalf("exit %d %s", code, stderr)
	}
}

// TestGrantVisibleToOtherSession runs ls as a separate broker process in a
// new session keyring, the way an agent sees a grant the human made.
func TestGrantVisibleToOtherSession(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN", "--for", "5m")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "ls", "--json")
	cmd.Env = append(os.Environ(), envBroker+"=1")
	var out []byte
	if err := keyringtest.InNewSession(func() (err error) {
		out, err = cmd.Output()
		return err
	}); err != nil {
		t.Fatalf("broker ls in another session: %v", err)
	}
	if !strings.Contains(string(out), `"name":"API_TOKEN","tier":"private"`) ||
		!strings.Contains(string(out), `"granted":true`) {
		t.Fatalf("ls in another session: %s", out)
	}
}
