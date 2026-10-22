// Package redact replaces secret values in a byte stream with
// [REDACTED:NAME] markers.
//
// The stream arrives in arbitrary chunks, so a value can be split across
// writes. Writer holds back only the bytes that could still start a value
// (at most the longest value's length minus one) and decides them once more
// input arrives or the stream ends. Ordinary output flows through at once.
package redact

import (
	"bytes"
	"errors"
	"io"
	"sort"
)

// MinLen is the shortest value redacted. Shorter values match too much
// ordinary output to be worth hiding.
const MinLen = 6

// Secret is a named value to redact.
type Secret struct {
	Name  string
	Value []byte
}

type entry struct {
	value  []byte
	marker []byte
}

// Writer redacts secrets from everything written to it and passes the rest
// to the underlying writer. Close flushes the held-back tail.
//
// At each position the longest value that matches wins. Matches that
// overlap (two values sharing bytes, or one value repeated with overlap)
// merge into one marker named for the leftmost match, so no byte of any
// occurrence of any value gets through.
type Writer struct {
	w io.Writer
	// byFirst lists the values starting with each byte, longest first.
	byFirst [256][]*entry
	entries []*entry
	// buf is input not yet decided; cover is how many bytes at its start
	// fall inside a match whose marker is already written.
	buf    []byte
	cover  int
	out    []byte
	closed bool
}

var errClosed = errors.New("redact: write after close")

// NewWriter returns a Writer that redacts secrets on their way to w.
// Values shorter than MinLen are not redacted. NewWriter copies the
// values, so the caller may clear its own.
func NewWriter(w io.Writer, secrets []Secret) *Writer {
	r := &Writer{w: w}
	sorted := append([]Secret(nil), secrets...)
	// Longest first; equal values go to the first name, deterministically.
	sort.SliceStable(sorted, func(i, j int) bool {
		if len(sorted[i].Value) != len(sorted[j].Value) {
			return len(sorted[i].Value) > len(sorted[j].Value)
		}
		return sorted[i].Name < sorted[j].Name
	})
	for _, s := range sorted {
		if len(s.Value) < MinLen || r.has(s.Value) {
			continue
		}
		e := &entry{
			value:  bytes.Clone(s.Value),
			marker: []byte("[REDACTED:" + s.Name + "]"),
		}
		r.entries = append(r.entries, e)
		r.byFirst[e.value[0]] = append(r.byFirst[e.value[0]], e)
	}
	return r
}

func (r *Writer) has(value []byte) bool {
	for _, e := range r.byFirst[value[0]] {
		if bytes.Equal(e.value, value) {
			return true
		}
	}
	return false
}

// Write redacts p and writes what it can decide so far. It reports len(p)
// unless the underlying writer fails.
func (r *Writer) Write(p []byte) (int, error) {
	if r.closed {
		return 0, errClosed
	}
	r.buf = append(r.buf, p...)
	if err := r.flush(false); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close writes the held-back tail, now that no more input can complete a
// value, and clears the Writer's copies of the values. It does not close
// the underlying writer.
func (r *Writer) Close() error {
	if r.closed {
		return nil
	}
	err := r.flush(true)
	r.closed = true
	for _, e := range r.entries {
		clear(e.value)
	}
	clear(r.buf[:cap(r.buf)])
	clear(r.out[:cap(r.out)])
	return err
}

// flush decides as much of buf as it can and writes the result. With final
// set the stream has ended, so a partial value is just output.
func (r *Writer) flush(final bool) error {
	buf, out := r.buf, r.out[:0]
	cover := r.cover
	// lit is where the pending run of literal output starts.
	i, lit := 0, 0
	for i < len(buf) {
		e, wait := r.match(buf[i:], final)
		if wait {
			break
		}
		if e != nil {
			if i >= cover {
				out = append(out, buf[lit:i]...)
				out = append(out, e.marker...)
			}
			cover = max(cover, i+len(e.value))
		}
		i++
		if i <= cover {
			lit = i
		}
	}
	out = append(out, buf[lit:i]...)

	n := copy(buf, buf[i:])
	clear(buf[n:])
	r.buf = buf[:n]
	r.cover = max(cover-i, 0)
	r.out = out
	if len(out) == 0 {
		return nil
	}
	_, err := r.w.Write(out)
	return err
}

// match returns the longest value at the start of s. wait reports that a
// longer value could still match once more input arrives.
func (r *Writer) match(s []byte, final bool) (e *entry, wait bool) {
	for _, e := range r.byFirst[s[0]] {
		if len(e.value) > len(s) {
			if !final && bytes.HasPrefix(e.value, s) {
				return nil, true
			}
			continue
		}
		if bytes.HasPrefix(s, e.value) {
			return e, false
		}
	}
	return nil, false
}
