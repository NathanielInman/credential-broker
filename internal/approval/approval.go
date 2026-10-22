// Package approval drives broker's request window: the desktop dialog in
// which a human approves an agent's grant request.
//
// The window is a separate program (broker-dialog) that speaks JSON lines:
// broker writes Messages to its stdin and reads Replies from its stdout.
// The window holds no logic beyond showing what it is told, so everything
// a human approves with is decided here, in broker.
package approval

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"filippo.io/age/plugin"
)

var (
	// ErrDenied means the human pressed Deny or closed the window.
	ErrDenied = errors.New("the human denied the request")
	// ErrTimeout means nobody answered before the request's deadline.
	ErrTimeout = errors.New("nobody answered the request in time")
)

// Secret is one secret the request asks for.
type Secret struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Request is what the window shows. broker works out everything but
// Reason, which is the agent's own word and shown as such.
type Request struct {
	Secrets []Secret `json:"secrets"`
	// For is the grant duration asked for, and Choices the durations the
	// human may pick instead (For and shorter), both in seconds.
	For     int64   `json:"for"`
	Choices []int64 `json:"choices"`
	// Deadline is when the request gives up.
	Deadline time.Time `json:"deadline"`
	// Ancestry is the requesting process chain, nearest first, and Cwd its
	// working directory, both read from /proc.
	Ancestry []string `json:"ancestry"`
	Cwd      string   `json:"cwd,omitempty"`
	// Home lets the window shorten paths to ~.
	Home string `json:"home,omitempty"`
	// Command is what exec --request runs once granted.
	Command []string `json:"command,omitempty"`
	// Zone is the vault's unlock zone. Approve is what approving takes:
	// "touch" (hardware is tried first), "passphrase", or "click" (a plain
	// vault, which unlocks with no human at all).
	Zone    string `json:"zone"`
	Approve string `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

// Message is one line from broker to the window.
type Message struct {
	// Type is "request", "prompt", "status", or "done".
	Type    string   `json:"type"`
	Request *Request `json:"request,omitempty"`
	Prompt  *Prompt  `json:"prompt,omitempty"`
	// Text is a status line, or done's outcome.
	Text string `json:"text,omitempty"`
	// Error marks a status or done Text as a failure.
	Error bool `json:"error,omitempty"`
	// Retry, on an error status, asks the human to press Approve to try
	// unlocking again.
	Retry bool `json:"retry,omitempty"`
}

// Prompt asks the human for a value (a passphrase or PIN) or, with Yes
// set, to choose between Yes and No.
type Prompt struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Secret bool   `json:"secret,omitempty"`
	Yes    string `json:"yes,omitempty"`
	No     string `json:"no,omitempty"`
}

// Reply is one line from the window to broker.
type Reply struct {
	// Type is "approve", "deny", "value" (an answer to Prompt ID), or
	// "for" (the human picked another duration).
	Type  string `json:"type"`
	ID    int    `json:"id,omitempty"`
	Value string `json:"value,omitempty"`
	Yes   bool   `json:"yes,omitempty"`
	For   int64  `json:"for,omitempty"`
}

// maxReply bounds one line from the window.
const maxReply = 64 << 10

// Session is one open request window.
type Session struct {
	req  Request
	cmd  *exec.Cmd
	in   io.WriteCloser
	send sync.Mutex
	enc  *json.Encoder

	approve chan struct{}
	values  chan Reply
	done    chan struct{}
	once    sync.Once
	err     error

	mu       sync.Mutex
	forSecs  int64
	promptID int
}

// Start runs the window program cmd and shows req in it until req's
// deadline.
func Start(cmd *exec.Cmd, req Request) (*Session, error) {
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start request window: %w", err)
	}
	s, err := New(out, in, req)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	s.cmd = cmd
	return s, nil
}

// New runs a session over a window already connected to r and w.
func New(r io.Reader, w io.WriteCloser, req Request) (*Session, error) {
	s := &Session{
		req:     req,
		in:      w,
		enc:     json.NewEncoder(w),
		approve: make(chan struct{}, 1),
		values:  make(chan Reply, 1),
		done:    make(chan struct{}),
		forSecs: req.For,
	}
	s.enc.SetEscapeHTML(false)
	if err := s.write(Message{Type: "request", Request: &req}); err != nil {
		return nil, fmt.Errorf("request window: %w", err)
	}
	go s.read(r)
	go func() {
		t := time.NewTimer(time.Until(req.Deadline))
		defer t.Stop()
		select {
		case <-t.C:
			s.end(ErrTimeout)
		case <-s.done:
		}
	}()
	return s, nil
}

func (s *Session) write(m Message) error {
	s.send.Lock()
	defer s.send.Unlock()
	return s.enc.Encode(m)
}

// read routes the window's replies until it closes, which counts as a
// deny: closing the window is how a human says no.
func (s *Session) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 4096), maxReply)
	for sc.Scan() {
		var rep Reply
		if json.Unmarshal(sc.Bytes(), &rep) != nil {
			continue
		}
		switch rep.Type {
		case "approve":
			select {
			case s.approve <- struct{}{}:
			default:
			}
		case "deny":
			s.end(ErrDenied)
			return
		case "value":
			select {
			case s.values <- rep:
			default:
			}
		case "for":
			s.pick(rep.For)
		}
	}
	s.end(ErrDenied)
}

// pick sets the grant duration to secs if it is one of the choices.
func (s *Session) pick(secs int64) {
	for _, c := range s.req.Choices {
		if c == secs {
			s.mu.Lock()
			s.forSecs = secs
			s.mu.Unlock()
			return
		}
	}
}

func (s *Session) end(err error) {
	s.once.Do(func() {
		s.err = err
		close(s.done)
	})
}

// Done is closed once the request is over without an approval: denied,
// the window closed, or the deadline passed. Err says which.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err is why Done closed.
func (s *Session) Err() error {
	<-s.done
	return s.err
}

// For is the grant duration the human settled on.
func (s *Session) For() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Duration(s.forSecs) * time.Second
}

// WaitApprove blocks until the human presses Approve, or the request ends.
func (s *Session) WaitApprove() error {
	select {
	case <-s.approve:
		return nil
	case <-s.done:
		return s.err
	}
}

// Status shows text under the request. With retry, it is an error and the
// window offers Approve again; WaitApprove then waits for that press.
func (s *Session) Status(text string, isErr, retry bool) {
	_ = s.write(Message{Type: "status", Text: text, Error: isErr, Retry: retry})
}

// Finish shows the outcome, waits briefly for the window to close itself,
// and ends the session.
func (s *Session) Finish(text string, isErr bool) {
	_ = s.write(Message{Type: "done", Text: text, Error: isErr})
	s.end(nil)
	if s.cmd == nil {
		_ = s.in.Close()
		return
	}
	exited := make(chan struct{})
	go func() {
		_ = s.cmd.Wait()
		close(exited)
	}()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-exited
	}
	_ = s.in.Close()
}

// UI is the ClientUI unlocking runs with: passphrase and PIN prompts, and
// plugin messages ("touch your YubiKey"), all go to the window.
func (s *Session) UI() *plugin.ClientUI {
	return &plugin.ClientUI{
		DisplayMessage: func(name, message string) error {
			s.Status(message, false, false)
			return nil
		},
		RequestValue: func(name, prompt string, secret bool) (string, error) {
			rep, err := s.ask(Prompt{Text: prompt, Secret: secret})
			return rep.Value, err
		},
		Confirm: func(name, prompt, yes, no string) (bool, error) {
			rep, err := s.ask(Prompt{Text: prompt, Yes: yes, No: no})
			return rep.Yes, err
		},
		WaitTimer: func(name string) {
			s.Status("waiting on the "+name+" plugin", false, false)
		},
	}
}

// ask shows p and waits for its answer, or the end of the request.
func (s *Session) ask(p Prompt) (Reply, error) {
	s.mu.Lock()
	s.promptID++
	p.ID = s.promptID
	s.mu.Unlock()
	if err := s.write(Message{Type: "prompt", Prompt: &p}); err != nil {
		return Reply{}, err
	}
	for {
		select {
		case rep := <-s.values:
			if rep.ID == p.ID {
				return rep, nil
			}
		case <-s.done:
			if s.err == nil {
				return Reply{}, ErrDenied
			}
			return Reply{}, s.err
		}
	}
}
