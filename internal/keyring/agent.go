package keyring

// The grant agent stands in for the kernel keyring where there is none
// (macOS). It is a broker process ("broker grantd") that holds grants in
// memory only, drops each one on a timer, and exits once it has held none
// for a while, so nothing outlives a grant on disk and no daemon lingers.
// Clients talk to it over a unix socket in broker's runtime directory, one
// JSON request and response per connection, and it answers only processes
// of its own user. A grant is readable by any process of that user, just as
// with the kernel keyring. macOS and Windows use it.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"
)

// maxRequest bounds one request; a value is at most vault.MaxValue.
const maxRequest = 1 << 20

type agentRequest struct {
	// Op is "add", "read", "lookup", "list", or "revoke".
	Op    string `json:"op"`
	Name  string `json:"name,omitempty"`
	Value []byte `json:"value,omitempty"`
	// TTL is a grant's duration in whole seconds.
	TTL int64 `json:"ttl,omitempty"`
}

type agentResponse struct {
	// Error is "not_granted", or any other failure's text.
	Error   string       `json:"error,omitempty"`
	Value   []byte       `json:"value,omitempty"`
	Expires int64        `json:"expires,omitempty"`
	Live    bool         `json:"live,omitempty"`
	Grants  []agentGrant `json:"grants,omitempty"`
}

type agentGrant struct {
	Name    string `json:"name"`
	Expires int64  `json:"expires"`
}

const notGranted = "not_granted"

// store is the agent's grants.
type store struct {
	mu     sync.Mutex
	grants map[string]*held
	// last is when the store last held a grant or answered a request.
	last time.Time
}

type held struct {
	value   []byte
	expires time.Time
	timer   *time.Timer
}

func (s *store) drop(name string, h *held) {
	clear(h.value)
	h.timer.Stop()
	delete(s.grants, name)
}

func (s *store) handle(req agentRequest) agentResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = time.Now()
	switch req.Op {
	case "add":
		if req.TTL < 1 {
			return agentResponse{Error: "grant duration is not positive"}
		}
		if h, ok := s.grants[req.Name]; ok {
			s.drop(req.Name, h)
		}
		expires := time.Now().Add(time.Duration(req.TTL) * time.Second)
		h := &held{value: bytes.Clone(req.Value), expires: expires}
		h.timer = time.AfterFunc(time.Until(expires), func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.grants[req.Name] == h {
				s.drop(req.Name, h)
			}
		})
		s.grants[req.Name] = h
		return agentResponse{Expires: expires.Unix()}
	case "read", "lookup":
		h, ok := s.grants[req.Name]
		if !ok || !time.Now().Before(h.expires) {
			return agentResponse{Error: notGranted}
		}
		res := agentResponse{Expires: h.expires.Unix()}
		if req.Op == "read" {
			res.Value = h.value
		}
		return res
	case "list":
		var res agentResponse
		for name, h := range s.grants {
			res.Grants = append(res.Grants, agentGrant{Name: name, Expires: h.expires.Unix()})
		}
		sort.Slice(res.Grants, func(i, j int) bool { return res.Grants[i].Name < res.Grants[j].Name })
		return res
	case "revoke":
		h, ok := s.grants[req.Name]
		if ok {
			s.drop(req.Name, h)
		}
		return agentResponse{Live: ok}
	}
	return agentResponse{Error: fmt.Sprintf("unknown op %q", req.Op)}
}

// idleFor reports how long the store has been empty and unasked, or 0
// while it holds grants.
func (s *store) idleFor() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.grants) > 0 {
		return 0
	}
	return time.Since(s.last)
}

// Serve answers grant requests on ln, from processes of this user only,
// until it has held no grants for idle. Grants live only in its memory.
func Serve(ln net.Listener, idle time.Duration) error {
	s := &store{grants: map[string]*held{}, last: time.Now()}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(idle / 4)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if s.idleFor() >= idle {
					ln.Close()
					return
				}
			}
		}
	}()
	defer close(done)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go serveConn(s, conn)
	}
}

func serveConn(s *store, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := peerIsUs(conn); err != nil {
		return
	}
	var req agentRequest
	if err := json.NewDecoder(io.LimitReader(conn, maxRequest)).Decode(&req); err != nil {
		return
	}
	res := s.handle(req)
	clear(req.Value) // add keeps its own copy
	_ = json.NewEncoder(conn).Encode(res)
}

// Client talks to the grant agent at Socket. Start, when set, launches the
// agent for an add that finds none running.
type Client struct {
	Socket string
	Start  func() error
}

// errNoAgent means nothing listens on the socket: no grants exist.
var errNoAgent = errors.New("no grant agent running")

func (c *Client) do(req agentRequest, start bool) (agentResponse, error) {
	conn, err := c.dial()
	if errors.Is(err, errNoAgent) && start && c.Start != nil {
		if err = c.Start(); err != nil {
			return agentResponse{}, fmt.Errorf("start grant agent: %w", err)
		}
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(25 * time.Millisecond) {
			if conn, err = c.dial(); err == nil || !errors.Is(err, errNoAgent) || time.Now().After(deadline) {
				break
			}
		}
	}
	if err != nil {
		return agentResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return agentResponse{}, err
	}
	var res agentResponse
	if err := json.NewDecoder(io.LimitReader(conn, maxRequest)).Decode(&res); err != nil {
		return agentResponse{}, fmt.Errorf("grant agent: %w", err)
	}
	return res, nil
}

func (c *Client) dial() (net.Conn, error) {
	// A missing socket, or a stale one nobody listens on, means no agent.
	if _, err := os.Stat(c.Socket); errors.Is(err, os.ErrNotExist) {
		return nil, errNoAgent
	}
	conn, err := net.DialTimeout("unix", c.Socket, 2*time.Second)
	if err != nil && (errors.Is(err, os.ErrNotExist) || refused(err)) {
		return nil, errNoAgent
	}
	return conn, err
}

func (res agentResponse) err() error {
	switch res.Error {
	case "":
		return nil
	case notGranted:
		return ErrNotGranted
	}
	return errors.New("grant agent: " + res.Error)
}

// Add grants name for ttl (rounded up to whole seconds), starting the
// agent if none runs, and returns when the grant expires.
func (c *Client) Add(name string, value []byte, ttl time.Duration) (time.Time, error) {
	secs := int64((ttl + time.Second - 1) / time.Second)
	if secs < 1 {
		return time.Time{}, fmt.Errorf("grant duration %s is not positive", ttl)
	}
	res, err := c.do(agentRequest{Op: "add", Name: name, Value: value, TTL: secs}, true)
	if err == nil {
		err = res.err()
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("grant %s: %w", name, err)
	}
	return time.Unix(res.Expires, 0), nil
}

// Read returns name's granted value, or ErrNotGranted.
func (c *Client) Read(name string) ([]byte, error) {
	res, err := c.do(agentRequest{Op: "read", Name: name}, false)
	if errors.Is(err, errNoAgent) {
		return nil, ErrNotGranted
	}
	if err != nil {
		return nil, err
	}
	return res.Value, res.err()
}

// Lookup returns name's live grant, or ErrNotGranted.
func (c *Client) Lookup(name string) (Grant, error) {
	res, err := c.do(agentRequest{Op: "lookup", Name: name}, false)
	if errors.Is(err, errNoAgent) {
		return Grant{}, ErrNotGranted
	}
	if err == nil {
		err = res.err()
	}
	if err != nil {
		return Grant{}, err
	}
	return Grant{Name: name, Expires: time.Unix(res.Expires, 0)}, nil
}

// List returns every live grant, sorted by name.
func (c *Client) List() ([]Grant, error) {
	res, err := c.do(agentRequest{Op: "list"}, false)
	if errors.Is(err, errNoAgent) {
		return nil, nil
	}
	if err == nil {
		err = res.err()
	}
	if err != nil {
		return nil, err
	}
	var out []Grant
	for _, g := range res.Grants {
		out = append(out, Grant{Name: g.Name, Expires: time.Unix(g.Expires, 0)})
	}
	return out, nil
}

// Revoke drops name's grant and reports whether it was live.
func (c *Client) Revoke(name string) (bool, error) {
	res, err := c.do(agentRequest{Op: "revoke", Name: name}, false)
	if errors.Is(err, errNoAgent) {
		return false, nil
	}
	if err == nil {
		err = res.err()
	}
	return res.Live, err
}

// AgentCommand is how Start launches the agent: this executable's grantd.
// Tests replace it.
var AgentCommand = func() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return exec.Command(exe, "grantd"), nil
}

// StartAgent launches the grant agent in a session of its own, detached
// from the caller, which may exit right after.
func StartAgent() error {
	cmd, err := AgentCommand()
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = detached()
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
