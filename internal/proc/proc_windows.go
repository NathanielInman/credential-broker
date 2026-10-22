package proc

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Run starts cmd and returns its exit code. Windows has no signals to
// forward: Ctrl+C and Ctrl+Break reach every process on the console, the
// child included, and closing the console reaches them all too. Broker
// catches them only so it outlives the child, whose output may still need
// redacting. Failures to start are *StartError. Once cmd has started, the
// only error is a failure copying its output (cmd.Wait's I/O errors).
func Run(cmd *exec.Cmd) (int, error) {
	sigs := make(chan os.Signal, 16)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 0, &StartError{Err: err}
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return 0, err
	}
	return cmd.ProcessState.ExitCode(), nil
}
