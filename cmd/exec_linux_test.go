package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestExecRedacts(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN", "DB_PASS")
	out, stderr, code := runStdin(t, "", "exec", "API_TOKEN", "DB_PASS", "API_USER", "--", "sh", "-c", `
test "$API_TOKEN" = token-value && test "$DB_PASS" = db-password && test "$API_USER" = nate || exit 9
printf '%s|%s|%s\n' "$API_TOKEN" "$DB_PASS" "$API_USER"
printf 'err %s' "$DB_PASS" >&2`)
	if code != 0 || out != "[REDACTED:API_TOKEN]|[REDACTED:DB_PASS]|nate\n" || stderr != "err [REDACTED:DB_PASS]" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, stderr)
	}
}

// TestExecRedactsStream sends many values through the pipe, so they land
// across the kernel's and the copier's read boundaries.
func TestExecRedactsStream(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	out, stderr, code := runStdin(t, "", "exec", "API_TOKEN", "--", "sh", "-c",
		`i=0; while [ $i -lt 3000 ]; do printf '%s-' "$API_TOKEN"; i=$((i+1)); done; printf tok`)
	want := strings.Repeat("[REDACTED:API_TOKEN]-", 3000) + "tok"
	if code != 0 || out != want || stderr != "" {
		t.Fatalf("exit %d stderr %q stdout ends %q", code, stderr, out[max(0, len(out)-60):])
	}
}

// TestExecTTYNotRedacted gives exec a terminal for stdout: a human is
// watching, so the value passes untouched.
func TestExecTTYNotRedacted(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	master, tty := openPTY(t)
	var stderr bytes.Buffer
	code := run("test", []string{"exec", "API_TOKEN", "--", "sh", "-c", `printf '%s\n' "$API_TOKEN"`},
		strings.NewReader(""), tty, &stderr)
	tty.Close()
	out, _ := io.ReadAll(master) // EIO once the terminal side is closed
	if code != 0 || !strings.Contains(string(out), "token-value") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, stderr.String())
	}
}

func openPTY(t *testing.T) (master, tty *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	tty, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	return master, tty
}

func TestExecStdinAndExitStatus(t *testing.T) {
	// Public secrets need no grant, so this runs without a keyring.
	seedVault(t)
	for _, tc := range []struct {
		script, stdin, out string
		code               int
	}{
		{`cat; printf ' %s' "$API_USER"`, "hello", "hello nate", 0},
		{"cat; exit 3", "in", "in", 3},
		{"exit 255", "", "", 255},
		{"kill -TERM $$", "", "", 128 + 15},
		{"kill -KILL $$", "", "", 128 + 9},
	} {
		t.Run(tc.script, func(t *testing.T) {
			out, stderr, code := runStdin(t, tc.stdin, "exec", "API_USER", "--", "sh", "-c", tc.script)
			if code != tc.code || out != tc.out || stderr != "" {
				t.Fatalf("exit %d stdout %q stderr %q; want %d %q", code, out, stderr, tc.code, tc.out)
			}
		})
	}
}

func TestExecNoGrant(t *testing.T) {
	grantVault(t)
	marker := filepath.Join(t.TempDir(), "ran")
	execMarker := func() (string, string, int) {
		return runStdin(t, "", "exec", "API_TOKEN", "API_USER", "DB_PASS", "--", "touch", marker)
	}

	out, stderr, code := execMarker()
	want := `{"error":"no_grant","names":["API_TOKEN","DB_PASS"],"ask_human":"broker grant API_TOKEN DB_PASS --for 1h"}` + "\n"
	if code != 77 || out != "" || stderr != want {
		t.Fatalf("exit %d stdout %q stderr %s", code, out, stderr)
	}

	mustRun(t, "grant", "API_TOKEN", "--for", "1s")
	_, stderr, code = execMarker()
	if code != 77 || !strings.Contains(stderr, `"names":["DB_PASS"],"ask_human":"broker grant DB_PASS --for 1h"`) {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	// An expired grant is no grant.
	mustRun(t, "grant", "DB_PASS", "--for", "1h")
	time.Sleep(2100 * time.Millisecond)
	_, stderr, code = execMarker()
	if code != 77 || !strings.Contains(stderr, `"names":["API_TOKEN"]`) {
		t.Fatalf("after expiry: exit %d stderr %s", code, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command ran without a grant: %v", err)
	}
}

func TestExecErrors(t *testing.T) {
	seedVault(t)
	plain := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
		code int
	}{
		// Unknown names come first: a grant would not fix them.
		{[]string{"NOPE", "API_TOKEN", "GONE", "--", "true"}, `{"error":"unknown_secret","names":["NOPE","GONE"]}` + "\n", 78},
		{[]string{"lower", "--", "true"}, `"error":"usage"`, 64},
		{[]string{"API_USER", "--", "broker-test-no-such-command"}, `"error":"command_not_found"`, 127},
		{[]string{"API_USER", "--", "/nonexistent/command"}, `"error":"command_not_found"`, 127},
		{[]string{"API_USER", "--", plain}, `"error":"cannot_exec"`, 126},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			_, stderr, code := runStdin(t, "", append([]string{"exec"}, tc.args...)...)
			if code != tc.code || !strings.Contains(stderr, tc.want) || errorCode(t, stderr) == "" {
				t.Fatalf("exit %d stderr %s; want %d %s", code, stderr, tc.code, tc.want)
			}
		})
	}
}

// TestExecValuesNotInArgvOrDisk has the child look for the value in its
// own and broker's command lines and in broker's environment, then checks
// nothing under BROKER_HOME holds it in the clear. The script must not
// spell out the value, since it is in the child's argv itself.
func TestExecValuesNotInArgvOrDisk(t *testing.T) {
	grantVault(t)
	home := os.Getenv("BROKER_HOME")
	mustRun(t, "grant", "API_TOKEN")
	_, stderr, code := runStdin(t, "", "exec", "API_TOKEN", "--", "sh", "-c", `
test "${API_TOKEN#token-}" = value || exit 8
for f in /proc/$$/cmdline /proc/$PPID/cmdline /proc/$PPID/environ; do
	grep -aqF -- "$API_TOKEN" "$f" && { echo "$f" >&2; exit 9; }
done
exit 0`)
	if code != 0 {
		t.Fatalf("exit %d (9: value in argv or broker's environment) %s", code, stderr)
	}
	err := filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if bytes.Contains(data, []byte("token-value")) {
			t.Errorf("%s holds the value", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestExecForwardsSignals runs broker as its own process, signals it, and
// checks the child gets the signal, its output is still redacted, and its
// exit status comes back.
func TestExecForwardsSignals(t *testing.T) {
	grantVault(t)
	mustRun(t, "grant", "API_TOKEN")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			name := map[syscall.Signal]string{syscall.SIGTERM: "TERM", syscall.SIGINT: "INT"}[sig]
			broker := exec.Command(exe, "exec", "API_TOKEN", "--", "sh", "-c",
				`trap 'echo caught $API_TOKEN; exit 7' `+name+`
echo ready
while :; do sleep 0.05; done`)
			broker.Env = append(os.Environ(), envBroker+"=1")
			// Its own process group, so it is never the terminal's
			// foreground group and SIGINT is forwarded too.
			broker.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var stderr bytes.Buffer
			broker.Stderr = &stderr
			stdout, err := broker.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := broker.Start(); err != nil {
				t.Fatal(err)
			}
			lines := bufio.NewScanner(stdout)
			if !lines.Scan() || lines.Text() != "ready" {
				broker.Process.Kill()
				t.Fatalf("child said %q; stderr %s", lines.Text(), stderr.String())
			}
			if err := broker.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if !lines.Scan() || lines.Text() != "caught [REDACTED:API_TOKEN]" {
				t.Fatalf("child said %q", lines.Text())
			}
			err = broker.Wait()
			if broker.ProcessState.ExitCode() != 7 {
				t.Fatalf("broker exit %v; stderr %s", err, stderr.String())
			}
		})
	}
}
