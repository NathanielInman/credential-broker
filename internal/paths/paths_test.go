package paths

import (
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
