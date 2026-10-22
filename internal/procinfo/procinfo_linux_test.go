package procinfo

import (
	"os"
	"strings"
	"testing"
)

func TestAncestry(t *testing.T) {
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

func TestExeAndCwd(t *testing.T) {
	self, _ := os.Executable()
	if exe, err := Exe(os.Getpid()); err != nil || exe != self {
		t.Errorf("Exe = %q, %v; want %q", exe, err, self)
	}
	wd, _ := os.Getwd()
	if cwd, err := Cwd(os.Getpid()); err != nil || cwd != wd {
		t.Errorf("Cwd = %q, %v; want %q", cwd, err, wd)
	}
}
