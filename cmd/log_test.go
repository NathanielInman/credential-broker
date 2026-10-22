package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/audit"
)

// auditLog returns the entries broker log --json prints.
func auditLog(t *testing.T, args ...string) []audit.Entry {
	t.Helper()
	out, stderr := mustRun(t, append([]string{"log", "--json"}, args...)...)
	if stderr != "" {
		t.Fatalf("log stderr %q", stderr)
	}
	var entries []audit.Entry
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		var e audit.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// summary is an entry's action, names, tier, outcome, and detail, for
// comparing against what a test did.
func summary(e audit.Entry) string {
	result := "ok"
	if !e.OK {
		result = e.Error
	}
	s := string(e.Action) + " " + strings.Join(e.Names, ",") + " " + e.Tier + " " + result
	if e.Cmd != "" {
		s += " cmd=" + e.Cmd
	}
	if e.For != "" {
		s += " for=" + e.For
	}
	return s
}

func summaries(entries []audit.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, summary(e))
	}
	return out
}

// assertNoValues fails if any value appears anywhere under BROKER_HOME's
// state directory.
func assertNoValues(t *testing.T, home string, values ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "state", "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if strings.Contains(string(data), v) {
			t.Errorf("audit log holds the value %q:\n%s", v, data)
		}
	}
}

func TestAuditLog(t *testing.T) {
	home := initVault(t)
	requireTTY = func(io.Reader, string) error { return nil }
	t.Cleanup(func() { requireTTY = defaultRequireTTY })

	runStdin(t, "", "init", "--identity", "whatever") // vault_exists, still logged
	if _, stderr, code := runStdin(t, "public-value-1", "set", "PUB", "--stdin", "--public"); code != 0 {
		t.Fatal(stderr)
	}
	if _, stderr, code := runStdin(t, "private-value-2", "set", "PRIV", "--stdin"); code != 0 {
		t.Fatal(stderr)
	}
	if out, _ := mustRun(t, "get", "PUB"); out != "public-value-1\n" {
		t.Fatalf("get PUB %q", out)
	}
	if out, _ := mustRun(t, "get", "PRIV"); out != "private-value-2\n" {
		t.Fatalf("get PRIV %q", out)
	}
	runStdin(t, "", "get", "NOPE")
	if out, _ := mustRun(t, "exec", "PUB", "--", "sh", "-c", `printf %s "$PUB"`); out != "public-value-1" {
		t.Fatalf("exec PUB %q", out)
	}
	runStdin(t, "", "exec", "PUB", "NOPE", "--", "true")
	mustRun(t, "rm", "PUB")
	runStdin(t, "", "set", "lower", "--stdin")

	entries := auditLog(t)
	want := []string{
		"init   ok",
		"init   vault_exists",
		"set PUB public ok",
		"set PRIV private ok",
		"get PUB public ok",
		"get PRIV private ok",
		"get NOPE  unknown_secret",
		"exec PUB public ok cmd=sh",
		"exec PUB,NOPE public unknown_secret cmd=true",
		"rm PUB public ok",
		"set lower  usage",
	}
	if got := summaries(entries); !slices.Equal(got, want) {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	cwd, _ := os.Getwd()
	comm, _ := os.ReadFile("/proc/self/comm")
	for _, e := range entries {
		if e.PID != os.Getpid() || e.Cwd != cwd || time.Since(e.TS) > time.Minute ||
			len(e.Ancestry) < 2 || e.Ancestry[0] != strings.TrimSpace(string(comm)) {
			t.Errorf("entry %+v", e)
		}
	}
	// log itself is not logged.
	if n := len(auditLog(t)); n != len(entries) {
		t.Errorf("log grew from %d to %d entries", len(entries), n)
	}
	assertNoValues(t, home, "public-value-1", "private-value-2")
	if fi, err := os.Stat(filepath.Join(home, "state", "log.jsonl")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("log file: %v %v", fi, err)
	}
}

// TestAuditLogGrants covers the keyring commands, and the line that
// matters most: exec denied for want of a grant.
func TestAuditLogGrants(t *testing.T) {
	grantVault(t)
	home := os.Getenv("BROKER_HOME")
	mustRun(t, "grant", "API_TOKEN", "API_USER", "--for", "2h")
	out, _ := mustRun(t, "exec", "API_TOKEN", "--", "sh", "-c", `printf %s "$API_TOKEN"`)
	if out != "[REDACTED:API_TOKEN]" {
		t.Fatalf("exec stdout %q", out)
	}
	mustRun(t, "exec", "API_TOKEN", "API_USER", "--", "true")
	_, stderr, code := runStdin(t, "", "exec", "DB_PASS", "API_TOKEN", "--", "env")
	if code != 77 || errorCode(t, stderr) != "no_grant" {
		t.Fatalf("exec DB_PASS: exit %d %s", code, stderr)
	}
	mustRun(t, "grant", "--all", "--for", "10m")
	mustRun(t, "get", "DB_PASS")
	mustRun(t, "revoke", "DB_PASS")
	mustRun(t, "revoke", "--all")
	mustRun(t, "rm", "DB_PASS")

	want := []string{
		"init   ok",
		"set API_TOKEN private ok",
		"set DB_PASS private ok",
		"set API_USER public ok",
		"grant API_TOKEN private ok for=2h",
		"exec API_TOKEN private ok cmd=sh",
		"exec API_TOKEN,API_USER mixed ok cmd=true",
		"exec DB_PASS,API_TOKEN private no_grant cmd=env",
		"grant API_TOKEN,DB_PASS private ok for=10m",
		"get DB_PASS private ok",
		"revoke DB_PASS  ok",
		"revoke API_TOKEN  ok",
		"rm DB_PASS private ok",
	}
	if got := summaries(auditLog(t)); !slices.Equal(got, want) {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := summaries(auditLog(t, "--name", "DB_PASS")); len(got) != 6 {
		t.Errorf("log --name DB_PASS:\n%s", strings.Join(got, "\n"))
	}
	assertNoValues(t, home, "token-value", "db-password")
}

func TestLogFilters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BROKER_HOME", home)
	if out, stderr := mustRun(t, "log"); out != "" || stderr != "" {
		t.Fatalf("empty log: stdout %q stderr %q", out, stderr)
	}

	file := filepath.Join(home, "state", "log.jsonl")
	old := time.Now().Add(-48 * time.Hour).UTC()
	for _, e := range []audit.Entry{
		{TS: old, Action: audit.Grant, Names: []string{"A"}, Tier: "private", For: "1h", OK: true,
			Ancestry: []string{"broker", "zsh", "alacritty"}},
		{Action: audit.Exec, Names: []string{"A", "B"}, Tier: "mixed", Error: "no_grant", Cmd: "curl",
			Ancestry: []string{"broker", "bash", "claude", "zsh"}},
		{Action: audit.Set, Names: []string{"B"}, Tier: "public", OK: true, Ancestry: []string{"broker"}},
	} {
		if err := audit.Append(file, e); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		args []string
		want []string
	}{
		{nil, []string{"grant A private ok for=1h", "exec A,B mixed no_grant cmd=curl", "set B public ok"}},
		{[]string{"--since", "24h"}, []string{"exec A,B mixed no_grant cmd=curl", "set B public ok"}},
		{[]string{"--name", "A"}, []string{"grant A private ok for=1h", "exec A,B mixed no_grant cmd=curl"}},
		{[]string{"--name", "A", "--since", "1h"}, []string{"exec A,B mixed no_grant cmd=curl"}},
		{[]string{"--name", "C"}, nil},
	} {
		if got := summaries(auditLog(t, tc.args...)); !slices.Equal(got, tc.want) {
			t.Errorf("log %v = %q, want %q", tc.args, got, tc.want)
		}
	}

	out, _ := mustRun(t, "log")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "TIME ") {
		t.Fatalf("human log:\n%s", out)
	}
	for i, want := range []string{
		"grant  ok              private  A    for 1h  zsh < alacritty",
		"exec   no_grant        mixed    A,B  curl    bash < claude < zsh",
		"set    ok              public   B    -       -",
	} {
		if !strings.HasSuffix(strings.Join(strings.Fields(lines[i+1])[2:], " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("human log line %q, want it to end %q", lines[i+1], want)
		}
	}

	// A torn line is skipped with a note, not fatal.
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"ts":"2026-`)
	f.Close()
	// and the next entry still lands whole.
	if err := audit.Append(file, audit.Entry{Action: audit.Rm, Names: []string{"B"}, OK: true}); err != nil {
		t.Fatal(err)
	}
	if out, stderr, code := runStdin(t, "", "log", "--name", "B", "--json"); code != 0 ||
		strings.Count(out, "\n") != 3 || !strings.Contains(out, `"action":"rm"`) ||
		stderr != "skipped 1 malformed lines in "+file+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, stderr)
	}

	for _, args := range [][]string{{"log", "--since", "-1h"}, {"log", "--name", "lower"}, {"log", "x"}} {
		if _, stderr, code := runStdin(t, "", args...); code != 64 || errorCode(t, stderr) != "usage" {
			t.Errorf("%v: exit %d %s", args, code, stderr)
		}
	}
}

// TestAuditLogUnwritable checks that a log broker cannot write never
// changes a command's outcome, only adds a warning.
func TestAuditLogUnwritable(t *testing.T) {
	home := initVault(t)
	// A directory where the log file should be: unwritable even for root.
	file := filepath.Join(home, "state", "log.jsonl")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}

	out, stderr, code := runStdin(t, "value-123", "set", "X", "--stdin")
	if code != 0 || out != "stored X in the private tier\n" || !strings.HasPrefix(stderr, "broker: audit log not written: ") {
		t.Fatalf("set: exit %d stdout %q stderr %q", code, out, stderr)
	}
	// Failures keep their JSON line, last on stderr.
	_, stderr, code = runStdin(t, "", "get", "NOPE")
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if code != 78 || len(lines) != 2 || !strings.Contains(lines[0], "audit log not written") ||
		lines[1] != `{"error":"unknown_secret","names":["NOPE"]}` {
		t.Fatalf("get: exit %d stderr %q", code, stderr)
	}
}
