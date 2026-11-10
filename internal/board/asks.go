package board

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

// Ask is a request open right now: the approver writes one while its
// window is up (or waiting behind another), and removes it when the
// request ends. It holds what the window shows and nothing else.
type Ask struct {
	Names    []string  `json:"names"`
	For      string    `json:"for"`
	Reason   string    `json:"reason,omitempty"`
	Cwd      string    `json:"cwd,omitempty"`
	Ancestry []string  `json:"ancestry,omitempty"`
	Cmd      string    `json:"cmd,omitempty"`
	Opened   time.Time `json:"opened"`
	Deadline time.Time `json:"deadline"`
	// Queued means another request's window is up and this one waits.
	Queued bool `json:"queued,omitempty"`
	// Extend marks a request to extend grants that end too soon.
	Extend bool `json:"extend,omitempty"`
	PID    int  `json:"pid"`
}

// AskFile is where the approver with pid keeps its open request.
func AskFile(dir string, pid int) string {
	return filepath.Join(dir, strconv.Itoa(pid)+".json")
}

// WriteAsk records a as open, replacing what was there for its pid.
func WriteAsk(dir string, a Ask) error {
	if err := paths.MkdirAll(dir); err != nil {
		return err
	}
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return paths.WriteFile(AskFile(dir, a.PID), b)
}

// RemoveAsk forgets pid's request.
func RemoveAsk(dir string, pid int) {
	_ = os.Remove(AskFile(dir, pid))
}

// ReadAsks returns the open requests, oldest first. A file whose request
// is long past its deadline, or whose approver is gone, was left behind by
// a killed approver: it is removed and skipped.
func ReadAsks(dir string, now time.Time, alive func(pid int) bool) []Ask {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Ask
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		file := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var a Ask
		if json.Unmarshal(b, &a) != nil || a.PID == 0 {
			_ = os.Remove(file)
			continue
		}
		if now.After(a.Deadline.Add(30*time.Second)) || (alive != nil && !alive(a.PID)) {
			_ = os.Remove(file)
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Opened.Before(out[j].Opened) })
	return out
}

// Dismissed is the set of missed requests (by their log timestamp) the
// human waved away, kept in the runtime directory so it ends with the
// session.
type Dismissed map[string]bool

func LoadDismissed(file string) Dismissed {
	d := Dismissed{}
	b, err := os.ReadFile(file)
	if err != nil {
		return d
	}
	var list []string
	if json.Unmarshal(b, &list) == nil {
		for _, k := range list {
			d[k] = true
		}
	}
	return d
}

// Dismiss adds keys to the file at file, keeping it to the newest 500.
func Dismiss(file string, keys ...string) error {
	d := LoadDismissed(file)
	for _, k := range keys {
		d[k] = true
	}
	list := make([]string, 0, len(d))
	for k := range d {
		list = append(list, k)
	}
	sort.Strings(list)
	if len(list) > 500 {
		list = list[len(list)-500:]
	}
	b, _ := json.Marshal(list)
	if err := paths.MkdirAll(filepath.Dir(file)); err != nil {
		return err
	}
	return paths.WriteFile(file, b)
}

// Locked is what the screen lock revoked, so the lamp can say so until
// the next grant.
type Locked struct {
	At    time.Time `json:"at"`
	Names []string  `json:"names"`
}

func LoadLocked(file string) (Locked, bool) {
	b, err := os.ReadFile(file)
	if err != nil {
		return Locked{}, false
	}
	var l Locked
	if json.Unmarshal(b, &l) != nil {
		return Locked{}, false
	}
	return l, true
}

func SaveLocked(file string, l Locked) error {
	if err := paths.MkdirAll(filepath.Dir(file)); err != nil {
		return err
	}
	b, _ := json.Marshal(l)
	return paths.WriteFile(file, b)
}

func ClearLocked(file string) {
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
}
