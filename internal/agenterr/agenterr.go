// Package agenterr defines the stable errors broker reports to agents.
//
// Every failure is written to stderr as a single JSON line with an "error"
// code, and broker exits with that code's exit status. Agents should branch
// on the code, not on the message.
package agenterr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

type Code string

const (
	// General is any failure without a more specific code.
	General Code = "error"
	// Usage is a bad flag, argument, or subcommand.
	Usage Code = "usage"
	// Unsupported means broker does not run on this platform.
	Unsupported Code = "unsupported_platform"
	// NotImplemented is a command that exists but has no implementation yet.
	NotImplemented Code = "not_implemented"
	// NeedsHuman means the command would prompt, but stdin is not a TTY.
	NeedsHuman Code = "needs_human"
	// NoGrant means a private secret was requested without a live grant.
	NoGrant Code = "no_grant"
	// UnknownSecret means a requested name is not in the vault.
	UnknownSecret Code = "unknown_secret"
)

// Exit statuses follow sysexits.h where one fits. exec passes the child's
// status through, so these only identify broker's own failures.
var exitCodes = map[Code]int{
	General:        1,
	Usage:          64, // EX_USAGE
	Unsupported:    69, // EX_UNAVAILABLE
	NotImplemented: 70, // EX_SOFTWARE
	NeedsHuman:     75, // EX_TEMPFAIL: retry once a human has acted
	NoGrant:        77, // EX_NOPERM
	UnknownSecret:  78,
}

// Error is the JSON line written to stderr.
type Error struct {
	Code     Code     `json:"error"`
	Names    []string `json:"names,omitempty"`
	AskHuman string   `json:"ask_human,omitempty"`
	Message  string   `json:"message,omitempty"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return string(e.Code) + ": " + e.Message
	}
	return string(e.Code)
}

// ExitCode is the process exit status for e.
func (e *Error) ExitCode() int {
	if c, ok := exitCodes[e.Code]; ok {
		return c
	}
	return 1
}

func New(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// From returns err as an *Error, wrapping anything else as General.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: General, Message: err.Error()}
}

// Write emits err to w as one JSON line and returns the exit status.
func Write(w io.Writer, err error) int {
	e := From(err)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(e)
	return e.ExitCode()
}

// RequireTTY returns a NeedsHuman error carrying askHuman unless f is a
// terminal. Call it before any prompt so agents never block on input.
func RequireTTY(f *os.File, askHuman string) error {
	if term.IsTerminal(int(f.Fd())) {
		return nil
	}
	return &Error{
		Code:     NeedsHuman,
		Message:  "this command needs a human at a terminal",
		AskHuman: askHuman,
	}
}
