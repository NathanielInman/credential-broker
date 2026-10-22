// Package proc runs a child process the way a shell runs a foreground
// command: signals sent to broker reach the child, broker outlives it so
// its output can still be redacted, and its exit status comes back as a
// shell reports it.
package proc

// StartError is a failure to start the child, as opposed to one while it
// ran.
type StartError struct {
	Err error
}

func (e *StartError) Error() string { return e.Err.Error() }

func (e *StartError) Unwrap() error { return e.Err }
