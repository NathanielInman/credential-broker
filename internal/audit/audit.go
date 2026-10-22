// Package audit appends one JSON line per broker action to the audit log,
// recording which process tree asked. It never sees secret values: an
// Entry has no field that could hold one.
package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/procinfo"
)

type Action string

const (
	Init    Action = "init"
	Set     Action = "set"
	Rm      Action = "rm"
	Grant   Action = "grant"
	Revoke  Action = "revoke"
	Get     Action = "get"
	Exec    Action = "exec"
	Import  Action = "import"
	Backup  Action = "backup"
	Restore Action = "restore"
	Request Action = "request"
)

// Entry is one line of the log.
type Entry struct {
	TS     time.Time `json:"ts"`
	Action Action    `json:"action"`
	Names  []string  `json:"names,omitempty"`
	// Tier is the names' tier, "mixed" when they span both, or empty when
	// none of them is known.
	Tier string `json:"tier,omitempty"`
	// For is a grant's duration.
	For string `json:"for,omitempty"`
	OK  bool   `json:"ok"`
	// Error is the agenterr code of a failure.
	Error string `json:"error,omitempty"`
	Cwd   string `json:"cwd"`
	// Cmd is exec's child argv[0], never its arguments.
	Cmd string `json:"cmd,omitempty"`
	// Reason is the agent's stated reason for a request.
	Reason   string   `json:"reason,omitempty"`
	PID      int      `json:"pid"`
	Ancestry []string `json:"ancestry"`
}

// lockWait bounds how long Append waits for another process to finish its
// line. A stuck holder (a stopped broker, say) must not stall every other
// command, and an O_APPEND write of one buffer does not interleave on local
// filesystems anyway; the lock covers the ones where it might.
const lockWait = time.Second

// Append writes e as one line to file, creating it and its directory as
// needed. It fills in TS, Cwd, PID, and Ancestry when they are unset. The
// line goes out in a single O_APPEND write under an exclusive flock, so
// concurrent brokers never interleave lines.
func Append(file string, e Entry) error {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	if e.Cwd == "" {
		e.Cwd, _ = os.Getwd()
	}
	if e.PID == 0 {
		e.PID = os.Getpid()
	}
	if e.Ancestry == nil {
		e.Ancestry = Ancestry(e.PID)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil { // Encode adds the newline
		return err
	}

	if err := paths.MkdirAll(filepath.Dir(file)); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, paths.FileMode)
	if err != nil {
		return err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Mode().Perm() != paths.FileMode {
		if err := f.Chmod(paths.FileMode); err != nil {
			return err
		}
	}
	// Closing f releases the lock.
	lock(f)
	line := buf.Bytes()
	// A line cut short (a full disk) must not swallow this one too.
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, fi.Size()-1); err == nil && last[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	if _, err := f.Write(line); err != nil {
		return err
	}
	return f.Close()
}

// Line is an entry read back from the log, with its raw JSON.
type Line struct {
	Entry
	Raw []byte
}

// Read returns the entries in file, oldest first, for which keep returns
// true (all of them when keep is nil), and the number of lines it skipped
// as malformed (a line cut short by a full disk, say). A missing file is
// an empty log.
func Read(file string, keep func(Entry) bool) ([]Line, int, error) {
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	return read(f, keep)
}

func read(r io.Reader, keep func(Entry) bool) ([]Line, int, error) {
	var out []Line
	var bad int
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 4<<20)
	for sc.Scan() {
		raw := sc.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil || e.Action == "" {
			bad++
			continue
		}
		if keep == nil || keep(e) {
			out = append(out, Line{Entry: e, Raw: bytes.Clone(raw)})
		}
	}
	return out, bad, sc.Err()
}

// Ancestry returns the command names of pid and each of its ancestors up
// to pid 1: ["broker","bash","claude",...]. See procinfo.Ancestry.
func Ancestry(pid int) []string { return procinfo.Ancestry(pid) }
