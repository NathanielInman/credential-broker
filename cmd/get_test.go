package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/NathanielInman/credential-broker/internal/keyring"
)

func TestGet(t *testing.T) {
	seedVault(t)
	// get never reads grants.
	orig := userKeyring
	userKeyring = func() (*keyring.Keyring, error) { return nil, errors.New("get must not use the keyring") }
	t.Cleanup(func() { userKeyring = orig })

	for _, tc := range []struct{ name, out string }{
		{"API_USER", "nate\n"},
		{"API_TOKEN", "token-value\n"},
	} {
		if out, stderr := mustRun(t, "get", tc.name); out != tc.out || stderr != "" {
			t.Errorf("get %s: stdout %q stderr %q", tc.name, out, stderr)
		}
	}

	_, stderr, code := runStdin(t, "", "get", "NOPE")
	if code != 78 || stderr != `{"error":"unknown_secret","names":["NOPE"]}`+"\n" {
		t.Fatalf("get unknown: exit %d %s", code, stderr)
	}
	_, stderr, code = runStdin(t, "", "get", "lower")
	if code != 64 || errorCode(t, stderr) != "usage" {
		t.Fatalf("get lower: exit %d %s", code, stderr)
	}
}

func TestGetRoundTrip(t *testing.T) {
	seedVault(t)
	for _, value := range []string{"multi\nline", "trailing newline\n"} {
		if _, stderr, code := runStdin(t, value+"\n", "set", "X", "--stdin"); code != 0 {
			t.Fatal(stderr)
		}
		out, _ := mustRun(t, "get", "X")
		if _, stderr, code := runStdin(t, out, "set", "Y", "--stdin"); code != 0 {
			t.Fatal(stderr)
		}
		if out2, _ := mustRun(t, "get", "Y"); out2 != out || out != value+"\n" {
			t.Errorf("value %q: get %q, after set from get %q", value, out, out2)
		}
	}
}

// TestGetPrivateNeedsTouch checks that private get always unlocks afresh:
// without a terminal it needs a human, even with a live grant.
func TestGetPrivateNeedsTouch(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	requireTTY = defaultRequireTTY
	_, stderr, code := runStdin(t, "", "get", "API_TOKEN")
	if code != 75 || !strings.Contains(stderr, `"error":"needs_human"`) ||
		!strings.Contains(stderr, `"ask_human":"broker get API_TOKEN"`) {
		t.Fatalf("exit %d %s", code, stderr)
	}
	// Public needs no terminal.
	if out, _ := mustRun(t, "get", "API_USER"); out != "nate\n" {
		t.Fatalf("get API_USER %q", out)
	}
}
