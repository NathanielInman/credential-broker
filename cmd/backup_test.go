package cmd

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/diceware"
)

const testPassphrase = "a long enough test passphrase"

// typePassphrases answers broker's hidden prompts with answers, in order,
// and fails the test if it prompts more often.
func typePassphrases(t *testing.T, answers ...string) {
	t.Helper()
	orig := readPassword
	t.Cleanup(func() { readPassword = orig })
	readPassword = func(_ *cobra.Command, prompt string) ([]byte, error) {
		if len(answers) == 0 {
			t.Errorf("unexpected prompt %q", prompt)
			return nil, io.EOF
		}
		a := answers[0]
		answers = answers[1:]
		return []byte(a), nil
	}
}

// freshHome switches to an empty BROKER_HOME and returns it with a new
// plain identity file.
func freshHome(t *testing.T) (home, identity string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("BROKER_HOME", home)
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identity = filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(identity, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, identity
}

func TestBackupRestore(t *testing.T) {
	seedVault(t)
	if _, stderr, code := runStdin(t, "token-value", "set", "API_TOKEN", "--stdin", "--desc", "deploy token"); code != 0 {
		t.Fatal(stderr)
	}
	file := filepath.Join(t.TempDir(), "vault.backup")
	typePassphrases(t, testPassphrase, testPassphrase)
	out, stderr := mustRun(t, "backup", "-o", file)
	if out != "backed up 3 secrets to "+file+"\n" || stderr != "" {
		t.Fatalf("backup: stdout %q stderr %q", out, stderr)
	}
	if fi, err := os.Stat(file); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("backup file: %v %v", fi, err)
	}
	before := ls(t)

	// A fresh home, unlocked by a different plain identity.
	home, identity := freshHome(t)
	typePassphrases(t, testPassphrase)
	out, _ = mustRun(t, "restore", file, "--identity", identity, "--insecure-plain-identity")
	if !strings.HasPrefix(out, "restored 3 secrets from "+file) || !strings.Contains(out, identity+" (plain)") {
		t.Errorf("restore output %q", out)
	}
	after := ls(t)
	if len(after) != len(before) {
		t.Fatalf("ls before %+v after %+v", before, after)
	}
	for i := range before {
		if after[i] != before[i] {
			t.Errorf("ls[%d] = %+v, want %+v", i, after[i], before[i])
		}
	}
	for name, want := range map[string]string{"API_TOKEN": "token-value", "DB_PASS": "db-password", "API_USER": "nate"} {
		if got, _ := mustRun(t, "get", name); got != want+"\n" {
			t.Errorf("get %s = %q", name, got)
		}
	}

	// Only the restore and the gets are in the new home's log.
	got := summaries(auditLog(t))
	if got[0] != "restore API_TOKEN,API_USER,DB_PASS mixed ok" {
		t.Errorf("log %q", got)
	}
	assertNoValues(t, home, "token-value", "db-password", testPassphrase)
}

func TestBackupGeneratePassphrase(t *testing.T) {
	home := initVault(t)
	requireTTY = func(io.Reader, string) error { return nil }
	t.Cleanup(func() { requireTTY = defaultRequireTTY })
	if _, stderr, code := runStdin(t, "nate", "set", "API_USER", "--public", "--stdin"); code != 0 {
		t.Fatal(stderr)
	}
	file := filepath.Join(t.TempDir(), "vault.backup")
	typePassphrases(t)
	_, stderr := mustRun(t, "backup", "-o", file, "--generate-passphrase", "--include-log")
	pass, ok := strings.CutPrefix(strings.Split(stderr, "\n")[0], "passphrase: ")
	words := strings.Split(pass, " ")
	if !ok || len(words) != 7 {
		t.Fatalf("stderr %q", stderr)
	}
	for _, w := range words {
		if !slices.Contains(diceware.Words, w) {
			t.Errorf("%q is not a diceware word", w)
		}
	}
	assertNoValues(t, home, pass)

	// The backup is a plain age file with a single scrypt recipient: what
	// `age -d FILE | tar -t` lists.
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	id, err := age.NewScryptIdentity(pass)
	if err != nil {
		t.Fatal(err)
	}
	r, err := age.Decrypt(f, id)
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, h.Name)
	}
	for _, want := range []string{"manifest.json", "config/config.json", "data/public.key", "data/private.key",
		"data/secrets/public/API_USER.age", "state/log.jsonl"} {
		if !slices.Contains(entries, want) {
			t.Errorf("backup lacks %s: %v", want, entries)
		}
	}

	// Restored with the backup's own unlock identity, into a home with no
	// log: the backup's log carries on, with the restore after it.
	freshHome(t)
	typePassphrases(t, pass)
	mustRun(t, "restore", file)
	got := summaries(auditLog(t))
	want := []string{"init   ok", "set API_USER public ok", "restore API_USER public ok"}
	if !slices.Equal(got, want) {
		t.Errorf("log %q, want %q", got, want)
	}
	if out, _ := mustRun(t, "get", "API_USER"); out != "nate\n" {
		t.Errorf("get %q", out)
	}
}

func TestBackupPassphraseChecks(t *testing.T) {
	seedVault(t)
	file := filepath.Join(t.TempDir(), "vault.backup")

	typePassphrases(t, "")
	_, stderr, code := runStdin(t, "", "backup", "-o", file)
	if code != 64 || !strings.Contains(stderr, "empty") {
		t.Errorf("empty: exit %d %s", code, stderr)
	}
	typePassphrases(t, testPassphrase, testPassphrase+"x")
	_, stderr, code = runStdin(t, "", "backup", "-o", file)
	if code != 64 || !strings.Contains(stderr, "do not match") {
		t.Errorf("mismatch: exit %d %s", code, stderr)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("failed backup wrote %s", file)
	}

	typePassphrases(t, "short", "short")
	_, stderr = mustRun(t, "backup", "-o", file)
	if !strings.Contains(stderr, "broker: warning: the passphrase is only 5 characters") {
		t.Errorf("no warning: %q", stderr)
	}
	_, stderr, code = runStdin(t, "", "backup")
	if code != 64 || errorCode(t, stderr) != "usage" {
		t.Errorf("no -o: exit %d %s", code, stderr)
	}
}

func TestBackupRestoreNeedHuman(t *testing.T) {
	seedVault(t)
	file := filepath.Join(t.TempDir(), "vault.backup")
	typePassphrases(t, testPassphrase, testPassphrase)
	mustRun(t, "backup", "-o", file)

	requireTTY = defaultRequireTTY
	_, stderr, code := runStdin(t, "", "backup", "-o", file)
	if code != 75 || !strings.Contains(stderr, `"ask_human":"broker backup -o `+file+`"`) {
		t.Errorf("backup: exit %d %s", code, stderr)
	}
	freshHome(t)
	_, stderr, code = runStdin(t, "", "restore", file)
	if code != 75 || !strings.Contains(stderr, `"ask_human":"broker restore `+file+`"`) {
		t.Errorf("restore: exit %d %s", code, stderr)
	}
}

func TestRestoreRefusesExistingVault(t *testing.T) {
	seedVault(t)
	file := filepath.Join(t.TempDir(), "vault.backup")
	typePassphrases(t, testPassphrase, testPassphrase)
	mustRun(t, "backup", "-o", file)

	// No prompt: it fails first.
	typePassphrases(t)
	_, stderr, code := runStdin(t, "", "restore", file)
	if code != 73 || errorCode(t, stderr) != "vault_exists" {
		t.Fatalf("exit %d %s", code, stderr)
	}
	if got := summaries(auditLog(t, "--name", "API_TOKEN")); got[len(got)-1] != "backup API_TOKEN,API_USER,DB_PASS mixed ok" {
		t.Errorf("log %q", got)
	}
	if got := summaries(auditLog(t)); got[len(got)-1] != "restore   vault_exists" {
		t.Errorf("log %q", got)
	}

	typePassphrases(t, "wrong")
	_, stderr, code = runStdin(t, "", "restore", file, "--force")
	if code != 1 || !strings.Contains(stderr, "wrong passphrase") {
		t.Fatalf("exit %d %s", code, stderr)
	}
}

// TestRestoreForceRevokesGrants checks that grants of a replaced vault do
// not outlive it.
func TestRestoreForceRevokesGrants(t *testing.T) {
	grantVault(t)
	file := filepath.Join(t.TempDir(), "vault.backup")
	typePassphrases(t, testPassphrase, testPassphrase)
	mustRun(t, "backup", "-o", file)
	mustRun(t, "grant", "API_TOKEN", "DB_PASS")
	if len(granted(t)) != 2 {
		t.Fatal("grant did not take")
	}
	typePassphrases(t, testPassphrase)
	out, _ := mustRun(t, "restore", file, "--force")
	if !strings.Contains(out, "revoked grants from the replaced vault: API_TOKEN, DB_PASS\n") {
		t.Errorf("restore output %q", out)
	}
	if g := granted(t); len(g) != 0 {
		t.Errorf("still granted after restore: %v", g)
	}
}
