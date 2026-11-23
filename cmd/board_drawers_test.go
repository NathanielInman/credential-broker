package cmd

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDrawerOpenedHidesTheOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the drawers are Linux layer-shell panels; Windows reports a refused socket differently")
	}
	// Unix socket paths have a short limit; t.TempDir() can be too long.
	dir, err := os.MkdirTemp("", "drawers")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	hidden := make(chan string, 4)
	for _, name := range []string{"broker-board", "tannoy", "roundhouse"} {
		name := name
		stop, err := listenDrawers(dir, name, func() { hidden <- name })
		if err != nil {
			t.Fatal(err)
		}
		defer stop()
	}
	// A socket left by a process that is gone.
	dead := filepath.Join(dir, "tablet.sock")
	ln, err := net.Listen("unix", dead)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()

	if got := drawerOpened(dir, "broker-board"); got != 2 {
		t.Fatalf("told %d drawers, want 2", got)
	}
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case n := <-hidden:
			seen[n] = true
		case <-time.After(time.Second):
			t.Fatalf("only %v hid", seen)
		}
	}
	if seen["broker-board"] {
		t.Fatal("Signal Box hid itself")
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatal("the dead socket was not removed")
	}
	if _, err := listenDrawers(dir, "tannoy", func() {}); err == nil {
		t.Fatal("a second tannoy listener was allowed")
	}
}
