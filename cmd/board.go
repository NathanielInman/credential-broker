package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/board"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// boardTabs are the panel's tabs, for --tab.
var boardTabs = []string{"live", "asks", "register", "vault"}

func newBarCmd() *cobra.Command {
	var once bool
	c := &cobra.Command{
		Use:   "bar",
		Short: "Print Signal Box's lamp for waybar, one JSON line per change",
		Long: "bar is waybar's custom module for Signal Box: it prints the lamp as\n" +
			"JSON whenever it changes (glyph, live count, class, tooltip) and runs\n" +
			"until waybar stops it. It never sees a secret's value. With\n" +
			"lock.on_screen_lock = \"revoke\" in board.toml it also drops every grant\n" +
			"when the screen locks.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			enc := json.NewEncoder(out)
			enc.SetEscapeHTML(false)
			var last []byte
			var lastAt time.Time
			wasLocked := false
			for {
				line, cfg, err := barLine(p)
				if err != nil {
					line = board.BarOutput{Text: "\U000F0766", Class: "error", Alt: "error", Tooltip: "Signal Box\n" + err.Error()}
				}
				b, _ := json.Marshal(line)
				if !bytes.Equal(b, last) || time.Since(lastAt) > time.Minute {
					if err := enc.Encode(line); err != nil {
						return nil // waybar went away
					}
					last, lastAt = b, time.Now()
				}
				if once {
					return nil
				}
				if cfg != nil && cfg.Lock.OnScreenLock == "revoke" {
					locked := screenLocked(cfg.Lock.Lockers)
					if locked && !wasLocked {
						lockRevoke(p)
					}
					wasLocked = locked
				}
				time.Sleep(time.Second)
			}
		},
	}
	c.Flags().BoolVar(&once, "once", false, "print one line and exit")
	return c
}

// barLine works out the lamp's line for waybar.
func barLine(p paths.Paths) (board.BarOutput, *board.Config, error) {
	snap, cfg, err := boardSnapshot(p, false)
	if err != nil {
		return board.BarOutput{}, cfg, err
	}
	return board.Bar(snap.Lamp, cfg), cfg, nil
}

// logCache keeps the parsed audit log between snapshots of the long
// running bar and panel, and reads it again only when the file changed.
var logCache struct {
	sync.Mutex
	size    int64
	mod     time.Time
	entries []audit.Entry
}

func cachedLog(file string) ([]audit.Entry, error) {
	logCache.Lock()
	defer logCache.Unlock()
	fi, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Size() == logCache.size && fi.ModTime().Equal(logCache.mod) {
		return logCache.entries, nil
	}
	lines, _, err := audit.Read(file, nil)
	if err != nil {
		return nil, err
	}
	entries := make([]audit.Entry, len(lines))
	for i, l := range lines {
		entries[i] = l.Entry
	}
	logCache.size, logCache.mod, logCache.entries = fi.Size(), fi.ModTime(), entries
	return entries, nil
}

// boardSnapshot gathers what Signal Box shows. siblings asks Roundhouse
// and Switchyard for their lamps, which only the open panel needs.
func boardSnapshot(p paths.Paths, siblings bool) (board.Snapshot, *board.Config, error) {
	cfg, err := board.LoadConfig(p.BoardConfig())
	if err != nil {
		return board.Snapshot{}, nil, agenterr.New(agenterr.Usage, "%v", err)
	}
	v, err := vault.Open(p)
	if err != nil {
		return board.Snapshot{}, cfg, err
	}
	secrets, err := v.List()
	if err != nil {
		return board.Snapshot{}, cfg, err
	}
	now := time.Now()
	in := board.Input{Now: now, Secrets: secrets, Config: cfg, Zone: string(v.Config().Zone()), Alive: brokerAlive}
	in.Host, _ = os.Hostname()
	if i := strings.IndexByte(in.Host, '.'); i > 0 {
		in.Host = in.Host[:i]
	}
	in.Home, _ = os.UserHomeDir()
	_, in.Device = approveBy(v.Config())
	if kr, err := userKeyring(); err == nil {
		grants, err := kr.List()
		if err == nil {
			for _, g := range grants {
				st, err := lookupGrant(kr, v, g.Name)
				if err != nil || (!st.Live && !st.Changed) {
					continue
				}
				gi := board.GrantIn{Name: g.Name, Changed: st.Changed}
				if st.Expires != nil {
					gi.Expires = *st.Expires
				}
				in.Grants = append(in.Grants, gi)
			}
		}
	}
	if in.Log, err = cachedLog(p.LogFile()); err != nil {
		return board.Snapshot{}, cfg, err
	}
	in.Asks = board.ReadAsks(p.AsksDir(), now, brokerAlive)
	in.Dismissed = board.LoadDismissed(p.BoardDismissed())
	if l, ok := board.LoadLocked(p.BoardLocked()); ok {
		// The lock note lasts until something is granted again.
		regranted := false
		for i := len(in.Log) - 1; i >= 0 && in.Log[i].TS.After(l.At); i-- {
			if e := in.Log[i]; e.OK && (e.Action == audit.Grant || e.Action == audit.Request) {
				regranted = true
				break
			}
		}
		if regranted || now.Sub(l.At) > 12*time.Hour {
			board.ClearLocked(p.BoardLocked())
		} else {
			in.Locked = &l
		}
	}
	if siblings {
		in.Siblings = siblingLamps()
	}
	return board.Build(in), cfg, nil
}

// screenLocked reports whether one of the screen lockers is running.
func screenLocked(lockers []string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, d := range dirs {
		if d.Name()[0] < '0' || d.Name()[0] > '9' {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(string(b))
		for _, l := range lockers {
			if comm == l {
				return true
			}
		}
	}
	return false
}

// lockRevoke drops every grant because the screen locked, and notes what
// it dropped for the lamp.
func lockRevoke(p paths.Paths) {
	out, err := runSelf("revoke", "--all")
	if err != nil {
		return
	}
	var names []string
	for _, l := range strings.Split(string(out), "\n") {
		if n, ok := strings.CutPrefix(l, "revoked "); ok {
			names = append(names, strings.TrimSpace(n))
		}
	}
	if len(names) > 0 {
		_ = board.SaveLocked(p.BoardLocked(), board.Locked{At: time.Now().UTC(), Names: names})
	}
}

// runSelf runs this broker with args and returns its stdout, so what the
// panel does is logged and checked exactly like the same command typed.
var runSelf = func(args ...string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		var ae struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(lastLineString(msg)), &ae) == nil && ae.Error != "" {
			msg = ae.Error
			if ae.Message != "" {
				msg += ": " + ae.Message
			}
		}
		if msg == "" {
			msg = err.Error()
		}
		return out, errors.New(msg)
	}
	return out, nil
}

func lastLineString(s string) string { return string(lastLine([]byte(s))) }

func newBoardCmd() *cobra.Command {
	var tab string
	var center, asJSON bool
	c := &cobra.Command{
		Use:   "board [--tab TAB]",
		Short: "Open or close the Signal Box panel from the bar icon",
		Long: "board toggles Signal Box, the panel behind the lamp in your bar: what is\n" +
			"live, the requests open or missed, the day's activity, and the vault's\n" +
			"loose ends. On Hyprland it springs out of the icon you clicked; elsewhere\n" +
			"it opens centered. --json prints what it shows instead, for scripts.\n" +
			"Settings are in board.toml beside config.json.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			if asJSON {
				snap, _, err := boardSnapshot(p, false)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetEscapeHTML(false)
				return enc.Encode(snap)
			}
			if tab != "" && !contains(boardTabs, tab) {
				return agenterr.New(agenterr.Usage, "--tab is %q, want one of %s", tab, strings.Join(boardTabs, ", "))
			}
			if runtime.GOOS != "linux" {
				return errors.New("the Signal Box panel runs on Linux with Wayland for now; macOS and Windows are planned. broker board --json works everywhere")
			}
			cfg, err := board.LoadConfig(p.BoardConfig())
			if err != nil {
				return agenterr.New(agenterr.Usage, "%v", err)
			}
			req := boardRequest{Cmd: "toggle", Tab: tab}
			if a, ok := board.HyprAnchor(cfg.Board.Namespace, center); ok {
				req.Anchor = &a
			}
			if _, err := boardCall(p.BoardSocket(), req); err == nil {
				return nil
			}
			if err := startBoardServer(p); err != nil {
				return err
			}
			_, err = boardCall(p.BoardSocket(), req)
			return err
		},
	}
	c.Flags().StringVar(&tab, "tab", "", "open on this tab: "+strings.Join(boardTabs, ", "))
	c.Flags().BoolVar(&center, "center", false, "open in the middle of the screen instead of at the icon")
	c.Flags().BoolVar(&asJSON, "json", false, "print what the panel shows as JSON and exit")
	c.AddCommand(newBoardServeCmd())
	return c
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// boardRequest is one line on the panel's socket. Its shape follows
// Roundhouse's and Switchyard's, so the siblings can hand over to it.
type boardRequest struct {
	Cmd    string        `json:"cmd"`
	Anchor *board.Anchor `json:"anchor,omitempty"`
	Tab    string        `json:"tab,omitempty"`
}

// boardCall sends req to the panel's server and returns its reply.
func boardCall(sock string, req boardRequest) ([]byte, error) {
	conn, err := net.DialTimeout("unix", sock, 300*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	b, _ := json.Marshal(req)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var r struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(line, &r) == nil && !r.OK {
		return line, errors.New(r.Error)
	}
	return line, nil
}

// startBoardServer starts "broker board serve" on its own, outside
// waybar's process tree, and waits for its socket.
func startBoardServer(p paths.Paths) error {
	if err := desktopSession(); err != nil {
		return err
	}
	if _, err := boardWindowPath(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := paths.MkdirAll(p.State); err != nil {
		return err
	}
	logf, err := os.OpenFile(filepath.Join(p.State, "board.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, paths.FileMode)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "board", "serve")
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = newSession()
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := boardCall(p.BoardSocket(), boardRequest{Cmd: "ping"}); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("the Signal Box panel did not start; see %s", filepath.Join(p.State, "board.log"))
}

// boardWindowPath finds broker-board beside broker, else on PATH.
func boardWindowPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if p, err := exec.LookPath(filepath.Join(filepath.Dir(exe), "broker-board")); err == nil {
			return p, nil
		}
	}
	p, err := exec.LookPath("broker-board")
	if err != nil {
		return "", errors.New("broker-board is not installed beside broker or on PATH")
	}
	return p, nil
}

// siblingSocket is where a sibling's daemon listens, the way its own docs
// say: $<NAME>_RUNTIME_DIR, else $XDG_RUNTIME_DIR/<name>.
func siblingSocket(name, env, file string) string {
	dir := os.Getenv(env)
	if dir == "" {
		rt := os.Getenv("XDG_RUNTIME_DIR")
		if rt == "" {
			return ""
		}
		dir = filepath.Join(rt, name)
	}
	return filepath.Join(dir, file)
}

var siblings = []struct{ name, env, file string }{
	{"switchyard", "SWITCHYARD_RUNTIME_DIR", "yard.sock"},
	{"roundhouse", "ROUNDHOUSE_RUNTIME_DIR", "rh.sock"},
}

// siblingSend sends one request to a sibling and returns its reply line,
// giving up after 300 ms like the siblings do with each other.
func siblingSend(sock string, req any) ([]byte, error) {
	conn, err := net.DialTimeout("unix", sock, 300*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
	b, _ := json.Marshal(req)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	return bufio.NewReader(io.LimitReader(conn, 4<<20)).ReadBytes('\n')
}

// siblingLamps asks the running siblings for the dot their link shows:
// Switchyard's aggregate phase, Roundhouse's lamp level.
func siblingLamps() map[string]string {
	out := map[string]string{}
	for _, s := range siblings {
		sock := siblingSocket(s.name, s.env, s.file)
		if sock == "" {
			continue
		}
		line, err := siblingSend(sock, map[string]string{"cmd": "snapshot"})
		if err != nil {
			continue
		}
		var r struct {
			Lamp      struct{ Level string } `json:"lamp"`
			Aggregate struct{ Phase string } `json:"aggregate"`
		}
		_ = json.Unmarshal(line, &r)
		level := r.Lamp.Level
		if s.name == "switchyard" {
			level = r.Aggregate.Phase
		}
		if level == "" {
			level = "idle"
		}
		out[s.name] = level
	}
	return out
}
