package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NathanielInman/credential-broker/internal/board"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

func barOnce(t *testing.T) board.BarOutput {
	t.Helper()
	out, _ := mustRun(t, "bar", "--once")
	var b board.BarOutput
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("bar output %q: %v", out, err)
	}
	return b
}

func boardJSON(t *testing.T) board.Snapshot {
	t.Helper()
	out, _ := mustRun(t, "board", "--json")
	var s board.Snapshot
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("board --json output %q: %v", out, err)
	}
	return s
}

func TestBarAndBoard(t *testing.T) {
	grantVault(t)
	if b := barOnce(t); b.Class != "quiet" || !strings.Contains(b.Tooltip, "Nothing live") {
		t.Fatalf("quiet bar: %+v", b)
	}
	mustRun(t, "grant", "API_TOKEN", "--for", "1h")
	b := barOnce(t)
	if b.Class != "live" || !strings.HasSuffix(b.Text, " 1") || !strings.Contains(b.Tooltip, "API_TOKEN ends in 59m") {
		t.Fatalf("live bar: %+v", b)
	}

	home := os.Getenv("BROKER_HOME")
	os.WriteFile(filepath.Join(home, "config", "board.toml"), []byte(`
[bar]
glyphs = "unicode"

[[bundle]]
name = "work"
secrets = ["API_*", "DB_PASS"]
key = "w"
`), 0o600)
	s := boardJSON(t)
	if len(s.Grants) != 1 || s.Grants[0].Names[0] != "API_TOKEN" || s.Grants[0].Total != 3600 {
		t.Fatalf("grants: %+v", s.Grants)
	}
	if len(s.Bundles) != 1 || strings.Join(s.Bundles[0].Names, ",") != "API_TOKEN,API_USER,DB_PASS" || s.Bundles[0].Live {
		t.Fatalf("bundles: %+v", s.Bundles)
	}
	if s.Vault.Total != 3 || s.Vault.Public != 1 {
		t.Fatalf("vault: %+v", s.Vault)
	}
	if b := barOnce(t); !strings.HasPrefix(b.Text, "●") {
		t.Fatalf("unicode glyph: %+v", b)
	}

	mustRun(t, "revoke", "--all")
	if b := barOnce(t); b.Class != "quiet" {
		t.Fatalf("after revoke: %+v", b)
	}

	os.WriteFile(filepath.Join(home, "config", "board.toml"), []byte("[lock]\non_screen_lock = \"sometimes\"\n"), 0o600)
	if b := barOnce(t); b.Class != "error" || !strings.Contains(b.Tooltip, "on_screen_lock") {
		t.Fatalf("bad config: %+v", b)
	}
	if _, stderr, code := runStdin(t, "", "board", "--json"); code == 0 || errorCode(t, stderr) != "usage" {
		t.Fatalf("board --json with a bad config: exit %d, %s", code, stderr)
	}
}

func TestBoardTabFlag(t *testing.T) {
	seedVault(t)
	if _, stderr, code := runStdin(t, "", "board", "--tab", "nope"); code == 0 || errorCode(t, stderr) != "usage" {
		t.Fatalf("board --tab nope: exit %d, %s", code, stderr)
	}
}

func TestRequestShowsOnBoard(t *testing.T) {
	grantVault(t)
	seen := filepath.Join(t.TempDir(), "asks")
	t.Setenv(envAsksSeen, seen)
	useWindow(t, "approve")
	mustRun(t, "request", "API_TOKEN", "--reason", "deploy the dev box")

	b, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	var a board.Ask
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &a); err != nil {
		t.Fatalf("open request file %q: %v", b, err)
	}
	if strings.Join(a.Names, ",") != "API_TOKEN" || a.Reason != "deploy the dev box" || a.Queued || a.PID != os.Getpid() {
		t.Fatalf("open request %+v", a)
	}
	// Gone once the request ended.
	files, _ := os.ReadDir(filepath.Join(os.Getenv("BROKER_HOME"), "run", "asks"))
	if len(files) != 0 {
		t.Fatalf("request file left behind: %v", files)
	}
	s := boardJSON(t)
	if len(s.Asks) != 0 || len(s.Grants) != 1 || s.Grants[0].Reason != "deploy the dev box" {
		t.Fatalf("board after the request: asks %+v grants %+v", s.Asks, s.Grants)
	}
}

func TestBoardActionsRunBroker(t *testing.T) {
	home := initVault(t)
	var ran [][]string
	orig := runSelf
	runSelf = func(args ...string) ([]byte, error) {
		ran = append(ran, args)
		if args[0] == "revoke" {
			return []byte("revoked A_TOKEN\nrevoked B_TOKEN\n"), nil
		}
		return nil, nil
	}
	t.Cleanup(func() { runSelf = orig })
	p, _ := paths.Resolve()
	s := &boardServer{p: p, stderr: io.Discard}

	s.act(boardAction{Action: "revoke", Names: []string{"A_TOKEN"}})
	s.act(boardAction{Action: "revoke_all"})
	s.act(boardAction{Action: "request", Names: []string{"A_TOKEN", "B_TOKEN"}, For: 5400, Reason: "From Signal Box: extend by an hour"})
	s.act(boardAction{Action: "request", Names: []string{"--all"}, For: 60})
	s.act(boardAction{Action: "dismiss", Keys: []string{"2026-11-10T12:04:00Z"}})

	want := []string{
		"revoke A_TOKEN",
		"revoke --all",
		"request A_TOKEN B_TOKEN --for 1h30m --reason From Signal Box: extend by an hour",
	}
	if len(ran) != len(want) {
		t.Fatalf("ran %q", ran)
	}
	for i, w := range want {
		if got := strings.Join(ran[i], " "); got != w {
			t.Errorf("run %d: %q, want %q", i, got, w)
		}
	}
	if d := board.LoadDismissed(filepath.Join(home, "run", "board-dismissed.json")); !d["2026-11-10T12:04:00Z"] {
		t.Errorf("dismissed: %v", d)
	}
}
