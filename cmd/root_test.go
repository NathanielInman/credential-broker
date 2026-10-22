package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runBroker runs broker in a fresh, empty BROKER_HOME.
func runBroker(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("BROKER_HOME", t.TempDir())
	return runStdin(t, "", args...)
}

// runStdin runs broker in the current BROKER_HOME with stdin as input.
func runStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run("test", args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func errorCode(t *testing.T, stderr string) string {
	t.Helper()
	if strings.Count(stderr, "\n") != 1 {
		t.Fatalf("stderr is not one line: %q", stderr)
	}
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &e); err != nil {
		t.Fatalf("stderr is not JSON: %q", stderr)
	}
	return e.Error
}

func TestHelpListsCommands(t *testing.T) {
	out, _, code := runBroker(t, "--help")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, name := range []string{"init", "set", "rm", "grant", "revoke", "get", "import", "backup", "restore", "log", "ls", "exec"} {
		if !strings.Contains(out, "\n  "+name+" ") {
			t.Errorf("help is missing %q:\n%s", name, out)
		}
	}
}

func TestStubsNotImplemented(t *testing.T) {
	for _, args := range [][]string{
		{"import", "env"},
	} {
		t.Run(args[0], func(t *testing.T) {
			_, stderr, code := runBroker(t, args...)
			if got := errorCode(t, stderr); got != "not_implemented" {
				t.Errorf("error %q, want not_implemented", got)
			}
			if code != 70 {
				t.Errorf("exit %d, want 70", code)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"nope"},
		{"set"},
		{"set", "X", "--public", "--private"},
		{"set", "lower"},
		{"ls", "--bogus"},
		{"exec", "X", "true"},
		{"exec", "X", "--"},
		{"exec", "--", "true"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, code := runBroker(t, args...)
			if got := errorCode(t, stderr); got != "usage" {
				t.Errorf("error %q, want usage", got)
			}
			if code != 64 {
				t.Errorf("exit %d, want 64", code)
			}
		})
	}
}
