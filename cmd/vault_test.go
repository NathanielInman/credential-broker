package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

// initVault creates a vault in a fresh BROKER_HOME unlocked by a plain
// identity, and returns BROKER_HOME.
func initVault(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("BROKER_HOME", home)
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(file, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runStdin(t, "", "init", "--identity", file, "--insecure-plain-identity"); code != 0 {
		t.Fatalf("init exit %d: %s", code, stderr)
	}
	return home
}

type lsJSON struct {
	Name        string  `json:"name"`
	Tier        string  `json:"tier"`
	Description string  `json:"description"`
	Granted     bool    `json:"granted"`
	Expires     *string `json:"expires"`
}

func ls(t *testing.T) []lsJSON {
	t.Helper()
	out, stderr, code := runStdin(t, "", "ls", "--json")
	if code != 0 {
		t.Fatalf("ls exit %d: %s", code, stderr)
	}
	var entries []lsJSON
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("ls --json output %q: %v", out, err)
	}
	return entries
}

func TestSetLsRm(t *testing.T) {
	initVault(t)
	if got := ls(t); got == nil || len(got) != 0 {
		t.Fatalf("empty vault ls = %#v, want []", got)
	}

	if _, stderr, code := runStdin(t, "token-value\n", "set", "API_TOKEN", "--stdin", "--desc", "deploy token"); code != 0 {
		t.Fatalf("set exit %d: %s", code, stderr)
	}
	if _, stderr, code := runStdin(t, "nate", "set", "API_USER", "--stdin", "--public"); code != 0 {
		t.Fatalf("set exit %d: %s", code, stderr)
	}
	// Overwrite without flags keeps the tier and description.
	if _, stderr, code := runStdin(t, "token-2", "set", "API_TOKEN", "--stdin"); code != 0 {
		t.Fatalf("set exit %d: %s", code, stderr)
	}

	got := ls(t)
	want := []lsJSON{
		{Name: "API_TOKEN", Tier: "private", Description: "deploy token"},
		{Name: "API_USER", Tier: "public"},
	}
	if len(got) != len(want) {
		t.Fatalf("ls = %+v", got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Tier != want[i].Tier ||
			got[i].Description != want[i].Description || got[i].Granted || got[i].Expires != nil {
			t.Errorf("ls[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	out, _, code := runStdin(t, "", "ls")
	if code != 0 || out == "" {
		t.Fatalf("human ls exit %d: %q", code, out)
	}

	// The value is the stdin minus one trailing newline.
	v, err := openVault()
	if err != nil {
		t.Fatal(err)
	}
	if val, err := v.Decrypt("API_USER", nil); err != nil || string(val) != "nate" {
		t.Fatalf("API_USER = %q, %v", val, err)
	}

	if _, stderr, code := runStdin(t, "", "rm", "API_USER"); code != 0 {
		t.Fatalf("rm exit %d: %s", code, stderr)
	}
	if got := ls(t); len(got) != 1 || got[0].Name != "API_TOKEN" {
		t.Fatalf("ls after rm = %+v", got)
	}
	_, stderr, code := runStdin(t, "", "rm", "API_USER")
	if errorCode(t, stderr) != "unknown_secret" || code != 78 {
		t.Fatalf("rm unknown: exit %d %s", code, stderr)
	}
}

func TestSetWithoutTTYNeedsHuman(t *testing.T) {
	initVault(t)
	_, stderr, code := runStdin(t, "value", "set", "X")
	if errorCode(t, stderr) != "needs_human" || code != 75 {
		t.Fatalf("exit %d %s", code, stderr)
	}
}

func TestSetEmptyValue(t *testing.T) {
	initVault(t)
	_, stderr, code := runStdin(t, "\n", "set", "X", "--stdin")
	if errorCode(t, stderr) != "usage" || code != 64 {
		t.Fatalf("exit %d %s", code, stderr)
	}
}

func TestNoVault(t *testing.T) {
	for _, args := range [][]string{
		{"ls"},
		{"set", "X", "--stdin"},
		{"rm", "X"},
	} {
		t.Run(args[0], func(t *testing.T) {
			_, stderr, code := runBroker(t, args...)
			if errorCode(t, stderr) != "no_vault" || code != 66 {
				t.Fatalf("exit %d %s", code, stderr)
			}
		})
	}
}

func TestInitErrors(t *testing.T) {
	initVault(t)
	_, stderr, code := runStdin(t, "", "init", "--identity", "whatever")
	if errorCode(t, stderr) != "vault_exists" || code != 73 {
		t.Fatalf("second init: exit %d %s", code, stderr)
	}

	_, stderr, code = runBroker(t, "init")
	if errorCode(t, stderr) != "usage" || code != 64 {
		t.Fatalf("init without identity: exit %d %s", code, stderr)
	}
}
