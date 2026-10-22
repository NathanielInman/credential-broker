package approval

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"
)

// window is the far end of a session: what broker-dialog sees.
type window struct {
	t   *testing.T
	in  *bufio.Scanner
	out *io.PipeWriter
}

// open starts a session for req against a fake window.
func open(t *testing.T, req Request) (*Session, *window) {
	t.Helper()
	toWindow, fromBroker := io.Pipe()
	toBroker, fromWindow := io.Pipe()
	w := &window{t: t, in: bufio.NewScanner(toWindow), out: fromWindow}
	t.Cleanup(func() { fromWindow.Close(); toWindow.Close() })

	ready := make(chan *Session, 1)
	go func() {
		s, err := New(toBroker, fromBroker, req)
		if err != nil {
			t.Error(err)
		}
		ready <- s
	}()
	if m := w.next(); m.Type != "request" || m.Request == nil || m.Request.Secrets[0].Name != "API_TOKEN" {
		t.Fatalf("first message %+v", m)
	}
	return <-ready, w
}

func (w *window) next() Message {
	w.t.Helper()
	if !w.in.Scan() {
		w.t.Fatal("window got no message")
	}
	var m Message
	if err := json.Unmarshal(w.in.Bytes(), &m); err != nil {
		w.t.Fatal(err)
	}
	return m
}

func (w *window) reply(r Reply) {
	w.t.Helper()
	data, _ := json.Marshal(r)
	if _, err := w.out.Write(append(data, '\n')); err != nil {
		w.t.Fatal(err)
	}
}

func testRequest() Request {
	return Request{
		Secrets:  []Secret{{Name: "API_TOKEN", Description: "the api"}},
		For:      3600,
		Choices:  []int64{900, 3600},
		Deadline: time.Now().Add(time.Minute),
		Approve:  "click",
	}
}

func TestApprove(t *testing.T) {
	s, w := open(t, testRequest())
	w.reply(Reply{Type: "approve"})
	if err := s.WaitApprove(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
		t.Fatal("approving ended the session")
	default:
	}
}

func TestDeny(t *testing.T) {
	s, w := open(t, testRequest())
	w.reply(Reply{Type: "deny"})
	if err := s.WaitApprove(); !errors.Is(err, ErrDenied) {
		t.Fatalf("got %v, want ErrDenied", err)
	}
}

func TestClosingTheWindowDenies(t *testing.T) {
	s, w := open(t, testRequest())
	w.out.Close()
	if err := s.Err(); !errors.Is(err, ErrDenied) {
		t.Fatalf("got %v, want ErrDenied", err)
	}
}

func TestTimeout(t *testing.T) {
	req := testRequest()
	req.Deadline = time.Now().Add(50 * time.Millisecond)
	s, _ := open(t, req)
	if err := s.WaitApprove(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
}

func TestPrompt(t *testing.T) {
	s, w := open(t, testRequest())
	got := make(chan string, 1)
	go func() {
		v, err := s.UI().RequestValue("broker", "passphrase for ~/id.age:", true)
		if err != nil {
			t.Error(err)
		}
		got <- v
	}()
	m := w.next()
	if m.Type != "prompt" || !m.Prompt.Secret || m.Prompt.Text != "passphrase for ~/id.age:" {
		t.Fatalf("prompt %+v", m)
	}
	// An answer to some other prompt is ignored.
	w.reply(Reply{Type: "value", ID: m.Prompt.ID + 1, Value: "stale"})
	w.reply(Reply{Type: "value", ID: m.Prompt.ID, Value: "correct horse"})
	if v := <-got; v != "correct horse" {
		t.Fatalf("got %q", v)
	}
}

func TestPromptEndsWithTheSession(t *testing.T) {
	s, w := open(t, testRequest())
	errc := make(chan error, 1)
	go func() {
		_, err := s.UI().RequestValue("broker", "pin:", true)
		errc <- err
	}()
	w.next()
	w.reply(Reply{Type: "deny"})
	if err := <-errc; !errors.Is(err, ErrDenied) {
		t.Fatalf("got %v, want ErrDenied", err)
	}
}

func TestPickDuration(t *testing.T) {
	s, w := open(t, testRequest())
	w.reply(Reply{Type: "for", For: 60}) // not offered: ignored
	w.reply(Reply{Type: "for", For: 900})
	w.reply(Reply{Type: "approve"})
	if err := s.WaitApprove(); err != nil {
		t.Fatal(err)
	}
	if d := s.For(); d != 15*time.Minute {
		t.Fatalf("For = %s, want 15m", d)
	}
}

func TestFinish(t *testing.T) {
	s, w := open(t, testRequest())
	go s.Finish("Granted for 1h", false)
	if m := w.next(); m.Type != "done" || m.Text != "Granted for 1h" || m.Error {
		t.Fatalf("done %+v", m)
	}
}
