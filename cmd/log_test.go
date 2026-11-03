package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/procinfo"
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
		result = orDash(e.Error)
	}
	s := string(e.Action) + " " + strings.Join(e.Names, ",") + " " + e.Tier + " " + result
	if e.Cmd != "" {
		s += " cmd=" + e.Cmd
	}
	if e.Status != nil {
		s += " status=" + strconv.Itoa(*e.Status)
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
		"exit PUB public ok cmd=sh status=0",
		"exec PUB,NOPE public unknown_secret cmd=true",
		"rm PUB public ok",
		"set lower  usage",
	}
	if got := summaries(entries); !slices.Equal(got, want) {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	cwd, _ := os.Getwd()
	self := procinfo.Ancestry(os.Getpid())
	for _, e := range entries {
		if e.PID != os.Getpid() || e.Cwd != cwd || time.Since(e.TS) > time.Minute ||
			len(e.Ancestry) < 2 || len(self) == 0 || e.Ancestry[0] != self[0] {
			t.Errorf("entry %+v", e)
		}
	}
	// log itself is not logged.
	if n := len(auditLog(t)); n != len(entries) {
		t.Errorf("log grew from %d to %d entries", len(entries), n)
	}
	assertNoValues(t, home, "public-value-1", "private-value-2")
	if fi, err := os.Stat(filepath.Join(home, "state", "log.jsonl")); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Errorf("log file: %v %v", fi, err)
	}
}

// TestAuditLogGrants covers the keyring commands, and the line that
// matters most: exec denied for want of a grant.
func TestAuditLogGrants(t *testing.T) {
	grantVault(t)
	home := os.Getenv("BROKER_HOME")
	mustRun(t, "grant", "API_TOKEN", "API_USER", "--for", "2h")
	mustRun(t, "grant", "API_USER")
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
	mustRun(t, "revoke", "DB_PASS")
	mustRun(t, "revoke", "--all")
	mustRun(t, "rm", "DB_PASS")

	want := []string{
		"init   ok",
		"set API_TOKEN private ok",
		"set DB_PASS private ok",
		"set API_USER public ok",
		"grant API_TOKEN private ok for=2h",
		"grant   ok",
		"exec API_TOKEN private ok cmd=sh",
		"exit API_TOKEN private ok cmd=sh status=0",
		"exec API_TOKEN,API_USER mixed ok cmd=true",
		"exit API_TOKEN,API_USER mixed ok cmd=true status=0",
		"exec DB_PASS,API_TOKEN private no_grant cmd=env",
		"grant API_TOKEN,DB_PASS private ok for=10m",
		"get DB_PASS private ok",
		"revoke DB_PASS private ok",
		"revoke   ok",
		"revoke API_TOKEN private ok",
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

// TestAuditLogExit checks the exit entry that follows each exec whose
// secrets were handed out: the child's status, how long it ran, and the
// exec's pid, so the two pair up.
func TestAuditLogExit(t *testing.T) {
	initVault(t)
	if _, stderr, code := runStdin(t, "v", "set", "PUB", "--stdin", "--public"); code != 0 {
		t.Fatal(stderr)
	}
	runStdin(t, "", "exec", "PUB", "--", "sh", "-c", "exit 3")
	runStdin(t, "", "exec", "PUB", "--", "sh", "-c", "sleep 0.2")
	runStdin(t, "", "exec", "PUB", "--", "broker-test-no-such-command")
	runStdin(t, "", "exec", "NOPE", "--", "true")

	entries := auditLog(t)
	want := []string{
		"init   ok",
		"set PUB public ok",
		"exec PUB public ok cmd=sh",
		"exit PUB public - cmd=sh status=3",
		"exec PUB public ok cmd=sh",
		"exit PUB public ok cmd=sh status=0",
		"exec PUB public ok cmd=broker-test-no-such-command",
		"exit PUB public command_not_found cmd=broker-test-no-such-command",
		"exec NOPE  unknown_secret cmd=true",
	}
	if got := summaries(entries); !slices.Equal(got, want) {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i, e := range entries {
		if e.Action != audit.Exit {
			if e.Took != "" {
				t.Errorf("%s entry has took %q", e.Action, e.Took)
			}
			continue
		}
		if start := entries[i-1]; e.PID != start.PID || e.Cwd != start.Cwd {
			t.Errorf("exit %+v does not pair with exec %+v", e, start)
		}
		took, err := time.ParseDuration(e.Took)
		if err != nil || took < 0 || took > time.Minute {
			t.Errorf("exit took %q: %v", e.Took, err)
		}
	}
	if took, _ := time.ParseDuration(entries[5].Took); took < 200*time.Millisecond {
		t.Errorf("sleep 0.2 took %s", entries[5].Took)
	}
}

func TestRunTime(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                                      "0s",
		1234567 * time.Nanosecond:              "1ms",
		999 * time.Millisecond:                 "999ms",
		1500 * time.Millisecond:                "2s",
		10*time.Minute + 3400*time.Millisecond: "10m3s",
	} {
		if got := runTime(d); got != want {
			t.Errorf("runTime(%s) = %q, want %q", d, got, want)
		}
	}
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
		{Action: audit.Exit, Names: []string{"B"}, Tier: "public", Cmd: "curl", Status: new(7), Took: "10m3s",
			Ancestry: []string{"broker", "bash", "claude", "zsh"}},
	} {
		if err := audit.Append(file, e); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		args []string
		want []string
	}{
		{nil, []string{"grant A private ok for=1h", "exec A,B mixed no_grant cmd=curl", "set B public ok",
			"exit B public - cmd=curl status=7"}},
		{[]string{"--since", "24h"}, []string{"exec A,B mixed no_grant cmd=curl", "set B public ok",
			"exit B public - cmd=curl status=7"}},
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
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "TIME ") {
		t.Fatalf("human log:\n%s", out)
	}
	for i, want := range []string{
		"grant  ok              private  A    for 1h  zsh < alacritty",
		"expire -               private  A    after 1h  -",
		"exec   no_grant        mixed    A,B  curl    bash < claude < zsh",
		"set    ok              public   B    -       -",
		"exit   exit 7          public   B    curl after 10m3s  bash < claude < zsh",
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
		strings.Count(out, "\n") != 4 || !strings.Contains(out, `"action":"rm"`) ||
		stderr != "skipped 1 malformed lines in "+file+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, stderr)
	}

	for _, args := range [][]string{{"log", "--since", "-1h"}, {"log", "--name", "lower"}, {"log", "x"}} {
		if _, stderr, code := runStdin(t, "", args...); code != 64 || errorCode(t, stderr) != "usage" {
			t.Errorf("%v: exit %d %s", args, code, stderr)
		}
	}
}

// TestLogComputedRows covers the rows broker log works out rather than
// reads: grants that expired, and execs still running.
func TestLogComputedRows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BROKER_HOME", home)
	file := filepath.Join(home, "state", "log.jsonl")
	now := time.Now().UTC()
	at := func(ago time.Duration) time.Time { return now.Add(-ago) }
	for _, e := range []audit.Entry{
		// A and B for 1h: B is revoked after 10m, A runs out.
		{TS: at(5 * time.Hour), Action: audit.Grant, Names: []string{"A", "B"}, Tier: "private", For: "1h", OK: true},
		{TS: at(5*time.Hour - 10*time.Minute), Action: audit.Revoke, Names: []string{"B"}, Tier: "private", OK: true},
		// C is granted again before its first grant runs out: one expiry,
		// the second grant's.
		{TS: at(4 * time.Hour), Action: audit.Request, Names: []string{"C"}, Tier: "private", For: "1h", OK: true},
		{TS: at(3*time.Hour + 30*time.Minute), Action: audit.Grant, Names: []string{"C"}, Tier: "private", For: "15m", OK: true},
		// A denied request and a removed secret leave no expiry.
		{TS: at(3 * time.Hour), Action: audit.Request, Names: []string{"D"}, Tier: "private", For: "1h", Error: "denied"},
		{TS: at(3 * time.Hour), Action: audit.Grant, Names: []string{"E"}, Tier: "private", For: "1h", OK: true},
		{TS: at(2*time.Hour + 30*time.Minute), Action: audit.Rm, Names: []string{"E"}, Tier: "private", OK: true},
		// Still live: no expiry yet.
		{TS: at(time.Minute), Action: audit.Grant, Names: []string{"F"}, Tier: "private", For: "1h", OK: true},
		// pid 100 finished, pid 200 is running, pid 300's broker is gone.
		{TS: at(50 * time.Second), Action: audit.Exec, Names: []string{"F"}, Tier: "private", Cmd: "a", OK: true, PID: 100},
		{TS: at(40 * time.Second), Action: audit.Exit, Names: []string{"F"}, Tier: "private", Cmd: "a", OK: true, PID: 100,
			Status: new(0), Took: "10s"},
		{TS: at(30 * time.Second), Action: audit.Exec, Names: []string{"F"}, Tier: "private", Cmd: "b", OK: true, PID: 200},
		{TS: at(20 * time.Second), Action: audit.Exec, Names: []string{"F"}, Tier: "private", Cmd: "c", OK: true, PID: 300},
		// A denied exec never ran, whatever its pid.
		{TS: at(10 * time.Second), Action: audit.Exec, Names: []string{"G"}, Tier: "private", Cmd: "d", Error: "no_grant", PID: 200},
	} {
		e.Ancestry = []string{"broker"}
		if err := audit.Append(file, e); err != nil {
			t.Fatal(err)
		}
	}
	brokerAlive = func(pid int) bool { return pid == 100 || pid == 200 }
	t.Cleanup(func() {
		brokerAlive = defaultBrokerAlive
	})

	rows := func(args ...string) []string {
		t.Helper()
		out, _ := mustRun(t, append([]string{"log"}, args...)...)
		var got []string
		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n")[1:] {
			// Drop the date and time.
			got = append(got, strings.Join(strings.Fields(line)[2:], " "))
		}
		return got
	}
	want := []string{
		"grant ok private A,B for 1h -",
		"revoke ok private B - -",
		// An expiry at the same time as an entry goes after it.
		"request ok private C for 1h -",
		"expire - private A after 1h -",
		"grant ok private C for 15m -",
		"expire - private C after 15m -",
		"request denied private D for 1h -",
		"grant ok private E for 1h -",
		"rm ok private E - -",
		"grant ok private F for 1h -",
		"exec ok private F a -",
		"exit ok private F a after 10s -",
		"exec ok private F b (running) -",
		"exec ok private F c -",
		"exec no_grant private G d -",
	}
	if got := rows(); !slices.Equal(got, want) {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got, want := rows("--name", "C"), []string{
		"request ok private C for 1h -",
		"grant ok private C for 15m -",
		"expire - private C after 15m -",
	}; !slices.Equal(got, want) {
		t.Errorf("log --name C:\n%s", strings.Join(got, "\n"))
	}
	// The expiry is filtered by its own time, not its grant's.
	if got := rows("--since", "3h20m", "--name", "C"); !slices.Equal(got, []string{"expire - private C after 15m -"}) {
		t.Errorf("log --since 3h20m --name C:\n%s", strings.Join(got, "\n"))
	}
	// Computed rows are never in --json.
	for _, e := range auditLog(t) {
		if e.Action == "expire" {
			t.Errorf("--json has a computed row %+v", e)
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
