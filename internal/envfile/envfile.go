// Package envfile reads NAME=value assignments from shell env files such as
// ~/.zshenv without running them.
//
// It understands the subset of POSIX shell (and zsh) that such files use for
// plain values: an optional export, single and double quotes, backslash
// escapes, line continuations, and trailing comments. Anything whose value
// only the shell could work out (a variable reference, command
// substitution, tilde) is returned with a Reason instead of a Value.
package envfile

import (
	"regexp"
	"strings"
)

// Assignment is one NAME=value statement.
type Assignment struct {
	Name string
	// Value is the value after quote removal, set only when Reason is empty.
	Value string
	// First and Last are the 1-based lines the statement spans.
	First, Last int
	// Reason says why the value cannot be read without running the shell,
	// or is empty when Value is the value.
	Reason string
}

// File is what Parse found in an env file.
type File struct {
	// Assignments are in file order.
	Assignments []Assignment
	// Refs maps each name a statement may expand ($NAME, ${NAME}) to the
	// lines doing so. Only statements kept in the file count: comments and
	// assignments with a known value expand nothing.
	Refs map[string][]int
}

// Reasons an assignment has no known value.
const (
	CommandSubstitution = "command substitution"
	Arithmetic          = "arithmetic expansion"
	VariableReference   = "variable reference"
	TildeExpansion      = "tilde expansion"
	ANSIQuote           = "$'...' quoting"
	Append              = "appends to an existing value (+=)"
	Unterminated        = "unterminated quote"
	Unsupported         = "unsupported shell syntax"
)

var refRE = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)

// Parse reads every assignment in src. It never fails: statements it does
// not understand are either not assignments (skipped) or assignments with a
// Reason.
func Parse(src string) File {
	src = strings.TrimPrefix(src, "\uFEFF")
	src = strings.ReplaceAll(src, "\r\n", "\n")
	s := &scanner{src: src, line: 1}
	f := File{Refs: map[string][]int{}}
	for !s.eof() {
		s.skipBlanks()
		switch s.peek() {
		case '\n':
			s.next()
			continue
		case '#':
			s.skipComment()
			s.next()
			continue
		}
		start, first := s.pos, s.line
		a, ok := s.statement()
		end := s.pos
		if ok {
			// Only a quote left open to the end of the file ends on a newline.
			a.First = first
			a.Last = first + strings.Count(strings.TrimSuffix(src[start:end], "\n"), "\n")
			f.Assignments = append(f.Assignments, a)
		}
		s.next() // the newline ending the statement
		if ok && a.Reason == "" {
			continue
		}
		text := src[start:end]
		for _, m := range refRE.FindAllStringSubmatchIndex(text, -1) {
			name := text[m[2]:m[3]]
			f.Refs[name] = append(f.Refs[name], first+strings.Count(text[:m[0]], "\n"))
		}
	}
	return f
}

type scanner struct {
	src  string
	pos  int
	line int
}

func (s *scanner) eof() bool { return s.pos >= len(s.src) }

// peek returns the next byte, or 0 at the end.
func (s *scanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.src[s.pos]
}

// next consumes and returns the next byte, or 0 at the end.
func (s *scanner) next() byte {
	if s.eof() {
		return 0
	}
	c := s.src[s.pos]
	s.pos++
	if c == '\n' {
		s.line++
	}
	return c
}

func (s *scanner) skipBlanks() {
	for c := s.peek(); c == ' ' || c == '\t'; c = s.peek() {
		s.next()
	}
}

// skipComment skips to the end of the line, leaving the newline.
func (s *scanner) skipComment() {
	for !s.eof() && s.peek() != '\n' {
		s.next()
	}
}

// statement parses one statement up to (not including) the newline that
// ends it. ok reports whether it was an assignment.
func (s *scanner) statement() (a Assignment, ok bool) {
	if strings.HasPrefix(s.src[s.pos:], "export") && isBlank(s.at(len("export"))) {
		s.pos += len("export")
		s.skipBlanks()
	}
	a.Name = s.name()
	switch {
	case a.Name != "" && s.peek() == '=':
		s.next()
		a.Value, a.Reason = s.word()
		if a.Reason == "" && !s.endOfStatement() {
			a.Reason = Unsupported
		}
	case a.Name != "" && strings.HasPrefix(s.src[s.pos:], "+="):
		a.Reason = Append
	default:
		// export NAME, a command, a function: not an assignment.
		s.skipStatement()
		return Assignment{}, false
	}
	s.skipStatement()
	if a.Reason != "" {
		a.Value = ""
	}
	return a, true
}

// at returns the byte off bytes ahead, or 0 past the end.
func (s *scanner) at(off int) byte {
	if s.pos+off >= len(s.src) {
		return 0
	}
	return s.src[s.pos+off]
}

func (s *scanner) name() string {
	start := s.pos
	for c := s.peek(); c == '_' || isAlpha(c) || (s.pos > start && isDigit(c)); c = s.peek() {
		s.next()
	}
	return s.src[start:s.pos]
}

// word reads one shell word and removes its quotes. It stops at an unquoted
// blank, newline, or operator, leaving it unread.
func (s *scanner) word() (value, reason string) {
	var b strings.Builder
	note := func(r string) {
		if reason == "" {
			reason = r
		}
	}
	// A tilde is expanded at the start of an assignment's value and after
	// each unquoted colon.
	tildeOK := true
	for {
		c := s.peek()
		if c == '~' && tildeOK {
			note(TildeExpansion)
		}
		tildeOK = c == ':'
		switch {
		case s.eof() || isBlank(c) || c == '\n' || strings.IndexByte(";&|<>()", c) >= 0:
			return b.String(), reason
		case c == '\'':
			s.next()
			if !s.until('\'', &b) {
				note(Unterminated)
			}
		case c == '"':
			s.next()
			if r := s.doubleQuoted(&b); r != "" {
				note(r)
			}
		case c == '\\':
			s.next()
			switch n := s.next(); n {
			case '\n': // line continuation
			case 0:
				b.WriteByte('\\')
			default:
				b.WriteByte(n)
			}
		case c == '$' && s.at(1) == '\'':
			s.next()
			s.next()
			s.ansiQuoted()
			note(ANSIQuote)
		case c == '$' || c == '`':
			if r := s.expansion(); r != "" {
				note(r)
			} else {
				b.WriteByte(s.next())
			}
		default:
			b.WriteByte(s.next())
		}
	}
}

// until copies bytes into b up to and consuming the closing quote q, and
// reports whether it found one.
func (s *scanner) until(q byte, b *strings.Builder) bool {
	for !s.eof() {
		c := s.next()
		if c == q {
			return true
		}
		if b != nil {
			b.WriteByte(c)
		}
	}
	return false
}

// doubleQuoted copies the inside of a "..." string into b, after the
// opening quote. Within it a backslash only escapes $ ` " \ and newline.
func (s *scanner) doubleQuoted(b *strings.Builder) (reason string) {
	note := func(r string) {
		if reason == "" {
			reason = r
		}
	}
	for {
		switch c := s.peek(); {
		case s.eof():
			note(Unterminated)
			return reason
		case c == '"':
			s.next()
			return reason
		case c == '\\':
			s.next()
			switch n := s.peek(); n {
			case '\n':
				s.next()
			case '$', '`', '"', '\\':
				b.WriteByte(s.next())
			default:
				b.WriteByte('\\')
			}
		case c == '$' || c == '`':
			if r := s.expansion(); r != "" {
				note(r)
			} else {
				b.WriteByte(s.next())
			}
		default:
			b.WriteByte(s.next())
		}
	}
}

// ansiQuoted skips the inside of a $'...' string, after the opening quote.
func (s *scanner) ansiQuoted() {
	for !s.eof() {
		switch s.next() {
		case '\\':
			s.next()
		case '\'':
			return
		}
	}
}

// expansion consumes the expansion starting at $ or ` and returns its
// reason, or returns "" without consuming anything when the $ is literal.
func (s *scanner) expansion() string {
	if s.peek() == '`' {
		s.next()
		for !s.eof() {
			switch s.next() {
			case '\\':
				s.next()
			case '`':
				return CommandSubstitution
			}
		}
		return CommandSubstitution
	}
	switch n := s.at(1); {
	case n == '(':
		reason := CommandSubstitution
		if s.at(2) == '(' {
			reason = Arithmetic
		}
		s.next()
		s.balanced('(', ')')
		return reason
	case n == '{':
		s.next()
		s.balanced('{', '}')
		return VariableReference
	case n == '_' || isAlpha(n):
		s.next()
		s.name()
		return VariableReference
	case isDigit(n) || strings.IndexByte("@*#?$!-", n) >= 0:
		s.next()
		s.next()
		return VariableReference
	}
	return ""
}

// balanced consumes from an opening bracket to its match, skipping quoted
// text, so a multi-line $(...) does not end the statement early.
func (s *scanner) balanced(open, close byte) {
	depth := 0
	for !s.eof() {
		switch c := s.next(); c {
		case open:
			depth++
		case close:
			if depth--; depth == 0 {
				return
			}
		case '\\':
			s.next()
		case '\'':
			s.until('\'', nil)
		case '"':
			s.doubleQuoted(&strings.Builder{})
		}
	}
}

// endOfStatement skips blanks and an optional ; and reports whether only a
// comment or the end of the line follows.
func (s *scanner) endOfStatement() bool {
	s.skipBlanks()
	if s.peek() == ';' {
		s.next()
		s.skipBlanks()
	}
	c := s.peek()
	return s.eof() || c == '\n' || c == '#'
}

// skipStatement consumes the rest of the statement, honouring quotes and
// expansions, and stops at the unquoted newline that ends it.
func (s *scanner) skipStatement() {
	for !s.eof() {
		c := s.peek()
		switch {
		case c == '\n':
			return
		case c == '#' && s.wordStart():
			s.skipComment()
			return
		case c == '\'':
			s.next()
			s.until('\'', nil)
		case c == '"':
			s.next()
			s.doubleQuoted(&strings.Builder{})
		case c == '\\':
			s.next()
			s.next()
		case c == '$' && s.at(1) == '\'':
			s.next()
			s.next()
			s.ansiQuoted()
		case c == '$' || c == '`':
			if s.expansion() == "" {
				s.next()
			}
		default:
			s.next()
		}
	}
}

// wordStart reports whether the next byte starts a word, where # begins a
// comment.
func (s *scanner) wordStart() bool {
	if s.pos == 0 {
		return true
	}
	prev := s.src[s.pos-1]
	return isBlank(prev) || prev == '\n' || strings.IndexByte(";&|()", prev) >= 0
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }
func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
