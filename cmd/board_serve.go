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
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/board"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

func newBoardServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "serve",
		Short:  "Run the Signal Box panel (started by broker board)",
		Hidden: true,
		Args:   usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			s := &boardServer{p: p, stderr: c.ErrOrStderr()}
			return s.serve()
		},
	}
}

// boardServer owns the panel: it listens on the board socket for
// toggles, keeps the panel's window program (broker-board) fed with
// snapshots while it shows, and carries out what the human does in it by
// running broker's own commands.
type boardServer struct {
	p      paths.Paths
	stderr io.Writer

	mu       sync.Mutex
	child    *exec.Cmd
	childIn  io.WriteCloser
	ready    chan struct{}
	visible  bool
	anchor   *board.Anchor
	hiddenAt time.Time
	lastSent []byte
	sibAt    time.Time
	sibs     map[string]string
}

func (s *boardServer) serve() error {
	sock := s.p.BoardSocket()
	if _, err := boardCall(sock, boardRequest{Cmd: "ping"}); err == nil {
		return errors.New("the Signal Box panel is already running")
	}
	if err := paths.MkdirAll(s.p.Run); err != nil {
		return err
	}
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer ln.Close()
	_ = os.Chmod(sock, paths.FileMode)
	go s.tick()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *boardServer) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	line, err := bufio.NewReader(io.LimitReader(conn, 1<<20)).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return
	}
	var req boardRequest
	reply := map[string]any{"ok": true}
	if err := json.Unmarshal(line, &req); err != nil {
		reply = map[string]any{"ok": false, "error": "bad request"}
	} else {
		switch req.Cmd {
		case "ping":
		case "snapshot":
			snap, _, err := boardSnapshot(s.p, false)
			if err != nil {
				reply = map[string]any{"ok": false, "error": err.Error()}
				break
			}
			reply["lamp"] = snap.Lamp
			reply["live"] = snap.Lamp.Count
			reply["asks"] = len(snap.Asks)
			reply["missed"] = len(snap.Missed)
		case "toggle":
			s.mu.Lock()
			vis := s.visible
			s.mu.Unlock()
			if vis {
				s.hide()
			} else if err := s.show(req.Anchor, req.Tab); err != nil {
				reply = map[string]any{"ok": false, "error": err.Error()}
			}
		case "show":
			if err := s.show(req.Anchor, req.Tab); err != nil {
				reply = map[string]any{"ok": false, "error": err.Error()}
			}
		case "hide":
			s.hide()
		case "board_visible":
			s.mu.Lock()
			reply["visible"] = s.visible
			s.mu.Unlock()
		default:
			reply = map[string]any{"ok": false, "error": "unknown cmd " + strconv.Quote(req.Cmd)}
		}
	}
	b, _ := json.Marshal(reply)
	_, _ = conn.Write(append(b, '\n'))
}

// send writes one message to the panel's window, if it is running.
func (s *boardServer) send(msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.childIn == nil {
		return
	}
	if _, err := s.childIn.Write(append(b, '\n')); err != nil {
		fmt.Fprintf(s.stderr, "board: write to window: %v\n", err)
	}
}

// ensureWindow starts broker-board if it is not running, and waits for
// its page to load.
func (s *boardServer) ensureWindow() error {
	s.mu.Lock()
	if s.child != nil {
		ready := s.ready
		s.mu.Unlock()
		<-ready
		return nil
	}
	path, err := boardWindowPath()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	cmd := exec.Command(path)
	in, err := cmd.StdinPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	cmd.Stderr = s.stderr
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return err
	}
	ready := make(chan struct{})
	s.child, s.childIn, s.ready, s.lastSent = cmd, in, ready, nil
	s.mu.Unlock()
	go s.readWindow(cmd, out, ready)

	cfg, _ := board.LoadConfig(s.p.BoardConfig())
	if cfg == nil {
		cfg = &board.Config{}
		cfg.Defaults()
	}
	home, _ := os.UserHomeDir()
	select {
	case <-ready:
	case <-time.After(8 * time.Second):
		return errors.New("the Signal Box window did not load")
	}
	s.send(map[string]any{"type": "init", "theme": board.LoadTheme(cfg, home, s.p.BoardCSS())})
	return nil
}

// readWindow handles what the panel's page says until the window exits.
func (s *boardServer) readWindow(cmd *exec.Cmd, out io.Reader, ready chan struct{}) {
	readyOnce := sync.Once{}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 4096), 1<<20)
	for sc.Scan() {
		var m boardAction
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		switch m.Type {
		case "ready":
			readyOnce.Do(func() { close(ready) })
		case "hidden":
			s.mu.Lock()
			s.visible, s.hiddenAt = false, time.Now()
			s.mu.Unlock()
		case "action":
			go s.act(m)
		}
	}
	_ = cmd.Wait()
	readyOnce.Do(func() { close(ready) })
	s.mu.Lock()
	if s.child == cmd {
		s.child, s.childIn, s.visible = nil, nil, false
	}
	s.mu.Unlock()
}

func (s *boardServer) show(a *board.Anchor, tab string) error {
	if err := s.ensureWindow(); err != nil {
		return err
	}
	s.mu.Lock()
	if a != nil {
		s.anchor = a
	}
	anchor := s.anchor
	s.visible = true
	s.lastSent = nil
	s.sibAt = time.Time{}
	s.mu.Unlock()
	s.push()
	s.send(map[string]any{"type": "show", "anchor": anchor, "tab": tab})
	// One panel at a time: the siblings close theirs.
	go func() {
		for _, sib := range siblings {
			if sock := siblingSocket(sib.name, sib.env, sib.file); sock != "" {
				_, _ = siblingSend(sock, map[string]string{"cmd": "hide"})
			}
		}
	}()
	return nil
}

func (s *boardServer) hide() {
	s.mu.Lock()
	s.visible, s.hiddenAt = false, time.Now()
	s.mu.Unlock()
	s.send(map[string]string{"type": "hide"})
}

// push sends a fresh snapshot to the page when it changed.
func (s *boardServer) push() {
	s.mu.Lock()
	needSibs := time.Since(s.sibAt) > 5*time.Second
	s.mu.Unlock()
	snap, _, err := boardSnapshot(s.p, false)
	if err != nil {
		s.send(map[string]any{"type": "error", "text": err.Error()})
		return
	}
	s.mu.Lock()
	if needSibs {
		s.mu.Unlock()
		sibs := siblingLamps()
		s.mu.Lock()
		s.sibs, s.sibAt = sibs, time.Now()
	}
	snap.Siblings = s.sibs
	s.mu.Unlock()
	// Now changes every time; compare without it.
	snap.Now = time.Time{}
	b, _ := json.Marshal(snap)
	s.mu.Lock()
	same := bytes.Equal(b, s.lastSent)
	s.lastSent = b
	s.mu.Unlock()
	if same {
		return
	}
	snap.Now = time.Now()
	s.send(map[string]any{"type": "snapshot", "snapshot": snap})
}

// tick refreshes the open panel every second and lets the hidden one go
// once it has lingered.
func (s *boardServer) tick() {
	for range time.Tick(time.Second) {
		s.mu.Lock()
		vis, child, hiddenAt := s.visible, s.child, s.hiddenAt
		s.mu.Unlock()
		if vis {
			s.push()
			continue
		}
		if child == nil {
			continue
		}
		linger := 10 * time.Minute
		if cfg, err := board.LoadConfig(s.p.BoardConfig()); err == nil {
			linger = cfg.Board.Linger.D()
		}
		if !hiddenAt.IsZero() && time.Since(hiddenAt) > linger {
			s.mu.Lock()
			if s.childIn != nil {
				_ = s.childIn.Close()
				s.childIn = nil
			}
			s.child = nil
			s.mu.Unlock()
		}
	}
}

// boardAction is a message from the panel's page.
type boardAction struct {
	Type    string   `json:"type"`
	Action  string   `json:"action"`
	ID      string   `json:"id"`
	Names   []string `json:"names"`
	For     int64    `json:"for"`
	Reason  string   `json:"reason"`
	Keys    []string `json:"keys"`
	Sibling string   `json:"sibling"`
}

// act carries out one thing the human did in the panel. Taking access
// away runs at once; anything that grants goes through broker request,
// so the request window (or the phone) asks for the touch as always.
func (s *boardServer) act(m boardAction) {
	result := func(ok bool, text string) {
		s.send(map[string]any{"type": "result", "id": m.ID, "ok": ok, "text": text})
		s.mu.Lock()
		s.lastSent = nil
		s.mu.Unlock()
		s.push()
	}
	for _, n := range m.Names {
		if err := validName(n); err != nil {
			result(false, err.Error())
			return
		}
	}
	switch m.Action {
	case "revoke":
		if len(m.Names) == 0 {
			return
		}
		out, err := runSelf(append([]string{"revoke"}, m.Names...)...)
		if err != nil {
			result(false, "Could not revoke: "+err.Error())
			return
		}
		result(true, summarize("Revoked", out, "revoked "))
	case "revoke_all":
		out, err := runSelf("revoke", "--all")
		if err != nil {
			result(false, "Could not revoke: "+err.Error())
			return
		}
		result(true, summarize("Revoked", out, "revoked "))
	case "request":
		if len(m.Names) == 0 || m.For <= 0 {
			return
		}
		dur := shortDuration(time.Duration(m.For) * time.Second)
		args := append([]string{"request"}, m.Names...)
		args = append(args, "--for", dur)
		if m.Reason != "" {
			args = append(args, "--reason", m.Reason)
		}
		_, err := runSelf(args...)
		if err != nil {
			result(false, "Not granted: "+err.Error())
			return
		}
		result(true, "Granted "+joinNames(m.Names)+" for "+dur)
	case "dismiss":
		if err := board.Dismiss(s.p.BoardDismissed(), m.Keys...); err != nil {
			result(false, err.Error())
			return
		}
		result(true, "Dismissed")
	case "sibling":
		for _, sib := range siblings {
			if sib.name != m.Sibling {
				continue
			}
			s.mu.Lock()
			a := s.anchor
			s.mu.Unlock()
			if _, err := siblingSend(siblingSocket(sib.name, sib.env, sib.file), map[string]any{"cmd": "show", "anchor": a}); err != nil {
				result(false, sib.name+" is not answering")
				return
			}
			s.hide()
		}
	}
}

func validName(n string) error {
	for _, r := range n {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return fmt.Errorf("invalid secret name %q", n)
		}
	}
	if n == "" || n[0] == '-' {
		return fmt.Errorf("invalid secret name %q", n)
	}
	return nil
}

// summarize turns "revoked A\nrevoked B" into "Revoked A, B".
func summarize(verb string, out []byte, prefix string) string {
	var names []string
	for _, l := range bytes.Split(out, []byte("\n")) {
		if n, ok := bytes.CutPrefix(l, []byte(prefix)); ok {
			names = append(names, string(bytes.TrimSpace(n)))
		}
	}
	if len(names) == 0 {
		return "Nothing was granted"
	}
	return verb + " " + joinNames(names)
}

func joinNames(names []string) string {
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", names[0], len(names)-1)
	}
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
