//go:build linux || darwin

package proc

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestExitStatus(t *testing.T) {
	for _, tc := range []struct {
		script string
		want   int
	}{
		{"exit 0", 0},
		{"exit 3", 3},
		{"exit 255", 255},
		{"kill -TERM $$", 128 + 15},
		{"kill -KILL $$", 128 + 9},
	} {
		t.Run(tc.script, func(t *testing.T) {
			status, err := Run(exec.Command("sh", "-c", tc.script))
			if err != nil || status != tc.want {
				t.Fatalf("Run = %d, %v; want %d", status, err, tc.want)
			}
		})
	}
}

func TestStartError(t *testing.T) {
	_, err := Run(exec.Command("/nonexistent/broker-test-command"))
	var start *StartError
	if !errors.As(err, &start) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Run = %v, want StartError wrapping ErrNotExist", err)
	}
}

// TestForwardsSignals signals this process while Run waits and checks the
// child gets the signal and broker survives it.
func TestForwardsSignals(t *testing.T) {
	sigs := []syscall.Signal{syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1}
	if !foreground() {
		// In the foreground the terminal delivers SIGINT itself, so Run
		// deliberately does not forward it.
		sigs = append(sigs, syscall.SIGINT)
	}
	for _, sig := range sigs {
		t.Run(sig.String(), func(t *testing.T) {
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer pr.Close()
			cmd := exec.Command("sh", "-c", `trap 'echo caught; exit 7' `+sigName(sig)+`
echo ready
while :; do sleep 0.05; done`)
			cmd.Stdout = pw
			type result struct {
				status int
				err    error
			}
			done := make(chan result, 1)
			go func() {
				status, err := Run(cmd)
				done <- result{status, err}
			}()

			lines := bufio.NewScanner(pr)
			if !lines.Scan() || lines.Text() != "ready" {
				t.Fatalf("child said %q", lines.Text())
			}
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			r := <-done
			pw.Close()
			if !lines.Scan() || lines.Text() != "caught" {
				t.Fatalf("child said %q", lines.Text())
			}
			if r.err != nil || r.status != 7 {
				t.Fatalf("Run = %d, %v; want 7", r.status, r.err)
			}
		})
	}
}

func sigName(sig syscall.Signal) string {
	return map[syscall.Signal]string{
		syscall.SIGTERM: "TERM", syscall.SIGHUP: "HUP", syscall.SIGUSR1: "USR1", syscall.SIGINT: "INT",
	}[sig]
}
