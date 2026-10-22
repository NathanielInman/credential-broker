package paths

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBrokerHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BROKER_HOME", root)
	t.Setenv("XDG_CONFIG_HOME", "/ignored")

	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := Paths{
		Config: filepath.Join(root, "config"),
		Data:   filepath.Join(root, "data"),
		State:  filepath.Join(root, "state"),
	}
	if p != want {
		t.Fatalf("got %+v, want %+v", p, want)
	}
	if got := p.TierDir("private"); got != filepath.Join(root, "data", "secrets", "private") {
		t.Fatalf("TierDir = %s", got)
	}
}

func TestResolveRelativeBrokerHome(t *testing.T) {
	t.Setenv("BROKER_HOME", "relative/dir")
	if _, err := Resolve(); err == nil {
		t.Fatal("want error for relative BROKER_HOME")
	}
}

func TestResolveXDG(t *testing.T) {
	t.Setenv("BROKER_HOME", "")
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_DATA_HOME", "relative")
	t.Setenv("XDG_STATE_HOME", "")

	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := Paths{
		Config: "/xdg/config/broker",
		Data:   "/home/u/.local/share/broker",
		State:  "/home/u/.local/state/broker",
	}
	if p != want {
		t.Fatalf("got %+v, want %+v", p, want)
	}
}

func TestEnsure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BROKER_HOME", root)
	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.Data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{p.Config, p.Data, p.State} {
		fi, err := os.Stat(d)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != DirMode {
			t.Errorf("%s mode %v, want %v", d, fi.Mode().Perm(), DirMode)
		}
	}
}

func TestWriteFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(name, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(name, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(name)
	if err != nil || string(got) != "new" {
		t.Fatalf("got %q, %v", got, err)
	}
	fi, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != FileMode {
		t.Errorf("mode %v, want %v", fi.Mode().Perm(), FileMode)
	}
	entries, _ := os.ReadDir(filepath.Dir(name))
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestWriteFileFuncFailure(t *testing.T) {
	name := filepath.Join(t.TempDir(), "f")
	if err := WriteFile(name, []byte("old")); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := WriteFileFunc(name, func(w io.Writer) error {
		io.WriteString(w, "partial")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
	if got, _ := os.ReadFile(name); string(got) != "old" {
		t.Fatalf("failed write left %q", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(name))
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}
