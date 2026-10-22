package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAppendRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state", "log.jsonl")
	if err := Append(file, Entry{Action: Get, Names: []string{"API_TOKEN"}, Tier: "private", OK: true}); err != nil {
		t.Fatal(err)
	}
	if err := Append(file, Entry{Action: Exec, Names: []string{"X"}, Error: "no_grant", Cmd: "curl"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{file: 0o600, filepath.Dir(file): 0o700} {
		fi, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s mode %v, want %v", name, fi.Mode().Perm(), want)
		}
	}

	lines, bad, err := Read(file, nil)
	if err != nil || bad != 0 || len(lines) != 2 {
		t.Fatalf("Read = %d lines, %d bad, %v", len(lines), bad, err)
	}
	e := lines[0].Entry
	cwd, _ := os.Getwd()
	if e.Action != Get || !e.OK || e.Tier != "private" || e.PID != os.Getpid() || e.Cwd != cwd ||
		time.Since(e.TS) > time.Minute || e.TS.Location() != time.UTC {
		t.Errorf("entry = %+v", e)
	}
	if e := lines[1].Entry; e.OK || e.Error != "no_grant" || e.Cmd != "curl" {
		t.Errorf("entry = %+v", e)
	}
	// Raw is the line as written, for log --json.
	var raw map[string]any
	if err := json.Unmarshal(lines[1].Raw, &raw); err != nil || raw["error"] != "no_grant" {
		t.Errorf("raw %s: %v", lines[1].Raw, err)
	}
	// Optional fields stay out of the line when empty.
	if _, ok := raw["for"]; ok {
		t.Errorf("raw has empty for: %s", lines[1].Raw)
	}

	only, _, _ := Read(file, func(e Entry) bool { return e.Action == Exec })
	if len(only) != 1 || only[0].Action != Exec {
		t.Errorf("filtered = %+v", only)
	}
}

func TestReadMissingAndMalformed(t *testing.T) {
	lines, bad, err := Read(filepath.Join(t.TempDir(), "nope"), nil)
	if lines != nil || bad != 0 || err != nil {
		t.Fatalf("missing file: %v %d %v", lines, bad, err)
	}
	in := `{"action":"get","ok":true}` + "\n\n" + `{"action":"se` + "\n" + `{"ok":true}` + "\n" + `{"action":"rm"}`
	lines, bad, err = read(strings.NewReader(in), nil)
	if err != nil || bad != 2 || len(lines) != 2 || lines[1].Action != Rm {
		t.Fatalf("read = %+v, %d bad, %v", lines, bad, err)
	}
}

func TestAppendTightensMode(t *testing.T) {
	file := filepath.Join(t.TempDir(), "log.jsonl")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(file, Entry{Action: Rm, OK: true}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
}

const envAppender = "AUDIT_TEST_APPENDER"

// TestConcurrentAppend appends long lines from several processes at once,
// each with several goroutines, and checks that every line comes back
// whole.
func TestConcurrentAppend(t *testing.T) {
	const procs, goroutines, perGoroutine = 6, 4, 25
	if file := os.Getenv(envAppender); file != "" {
		appendMany(t, file, goroutines, perGoroutine)
		return
	}
	file := filepath.Join(t.TempDir(), "log.jsonl")
	var cmds []*exec.Cmd
	for i := 0; i < procs; i++ {
		c := exec.Command(os.Args[0], "-test.run=^TestConcurrentAppend$")
		c.Env = append(os.Environ(), envAppender+"="+file)
		var out bytes.Buffer
		c.Stdout, c.Stderr = &out, &out
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if t.Failed() {
				t.Log(out.String())
			}
		})
		cmds = append(cmds, c)
	}
	for _, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Fatalf("appender: %v", err)
		}
	}

	lines, bad, err := Read(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := procs * goroutines * perGoroutine; bad != 0 || len(lines) != want {
		t.Fatalf("%d lines, %d malformed, want %d whole", len(lines), bad, want)
	}
	for _, l := range lines {
		if len(l.Names) != 400 {
			t.Fatalf("line has %d names, want 400", len(l.Names))
		}
	}
}

func appendMany(t *testing.T, file string, goroutines, n int) {
	// Lines several pages long, so a torn write would show.
	names := make([]string, 400)
	for i := range names {
		names[i] = fmt.Sprintf("NAME_%03d_%s", i, strings.Repeat("X", 20))
	}
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				if err := Append(file, Entry{Action: Exec, Names: names, OK: true}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestAncestry(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs /proc")
	}
	chain := Ancestry(os.Getpid())
	comm, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) < 2 || chain[0] != strings.TrimSpace(string(comm)) {
		t.Fatalf("ancestry %q, want it to start with %q", chain, comm)
	}
	// It runs up to pid 1, or as far up as this pid namespace goes.
	stat, _ := os.ReadFile("/proc/1/stat")
	if init, _, err := parseStat(stat); err == nil && chain[len(chain)-1] != init {
		t.Errorf("ancestry %q does not end at pid 1 (%s)", chain, init)
	}
	if Ancestry(-1) != nil {
		t.Error("Ancestry(-1) is not nil")
	}
}

func TestParseStat(t *testing.T) {
	for _, tc := range []struct {
		stat string
		comm string
		ppid int
	}{
		{"123 (bash) S 45 123 123 0", "bash", 45},
		{"123 (a) b (c)) R 7 1 1", "a) b (c)", 7},
		{"1 (systemd) S 0 1 1", "systemd", 0},
	} {
		comm, ppid, err := parseStat([]byte(tc.stat))
		if err != nil || comm != tc.comm || ppid != tc.ppid {
			t.Errorf("parseStat(%q) = %q, %d, %v", tc.stat, comm, ppid, err)
		}
	}
	for _, bad := range []string{"", "123 bash S 4", "123 (bash) S", "123 (bash) S x"} {
		if _, _, err := parseStat([]byte(bad)); err == nil {
			t.Errorf("parseStat(%q) did not fail", bad)
		}
	}
}
