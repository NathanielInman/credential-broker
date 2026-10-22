package procinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAncestryUsesArgv0(t *testing.T) {
	chain := Ancestry(os.Getpid())
	if len(chain) < 2 || chain[0] != filepath.Base(os.Args[0]) {
		t.Fatalf("ancestry %q, want it to start with %q", chain, filepath.Base(os.Args[0]))
	}
	if chain[len(chain)-1] != "launchd" {
		t.Errorf("ancestry %q does not end at launchd", chain)
	}
}

func TestExeAndCwd(t *testing.T) {
	self, _ := os.Executable()
	exe, err := Exe(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.Stat(exe)
	b, _ := os.Stat(self)
	if a == nil || b == nil || !os.SameFile(a, b) {
		t.Errorf("Exe = %q, want %q", exe, self)
	}
	wd, _ := os.Getwd()
	if cwd, err := Cwd(os.Getpid()); err != nil || !sameDir(cwd, wd) {
		t.Errorf("Cwd = %q, %v; want %q", cwd, err, wd)
	}
}

func sameDir(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}
