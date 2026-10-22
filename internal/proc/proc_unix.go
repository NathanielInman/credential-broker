package proc

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// forwarded are the signals passed on to the child. Broker catches them
// so it stays alive until the child exits. SIGTSTP and SIGCONT keep their
// defaults, so job control stops and resumes broker along with the child.
var forwarded = []os.Signal{
	unix.SIGINT, unix.SIGQUIT, unix.SIGTERM, unix.SIGHUP, unix.SIGUSR1, unix.SIGUSR2,
}

// Run starts cmd, forwards signals to it until it exits, and returns its
// exit status: its exit code, or 128+N when signal N killed it. Failures
// to start are *StartError. Once cmd has started, the only error is a
// failure copying its output (cmd.Wait's I/O errors).
func Run(cmd *exec.Cmd) (int, error) {
	sigs := make(chan os.Signal, 16)
	for _, s := range forwarded {
		// A signal broker was started ignoring (nohup, a background job
		// of a non-interactive shell) stays ignored, as it does for the
		// child.
		if !signal.Ignored(s) {
			signal.Notify(sigs, s)
		}
	}
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 0, &StartError{Err: err}
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				// The terminal already sent its keyboard signals to the
				// whole foreground process group, child included; sending
				// SIGINT twice makes some tools force quit.
				if (s == unix.SIGINT || s == unix.SIGQUIT) && foreground() {
					continue
				}
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)

	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return 0, err
	}
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ws.Signaled() {
		return 128 + int(ws.Signal()), nil
	}
	return ws.ExitStatus(), nil
}

// foreground reports whether broker's process group is the foreground
// group of its controlling terminal, so keyboard signals reach the child
// directly.
func foreground() bool {
	fd, err := unix.Open("/dev/tty", unix.O_RDONLY|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return false
	}
	defer unix.Close(fd)
	pgrp, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	return err == nil && pgrp == unix.Getpgrp()
}
