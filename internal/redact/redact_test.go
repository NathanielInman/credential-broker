package redact

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"
)

var tests = []struct {
	name    string
	secrets []Secret
	in      string
	want    string
}{
	{"none", nil, "plain output\n", "plain output\n"},
	{"one", []Secret{{"TOKEN", []byte("s3cr3t-token")}},
		"auth s3cr3t-token ok\n", "auth [REDACTED:TOKEN] ok\n"},
	{"many times", []Secret{{"TOKEN", []byte("s3cr3t")}},
		"s3cr3ts3cr3t s3cr3t\ns3cr3t", "[REDACTED:TOKEN][REDACTED:TOKEN] [REDACTED:TOKEN]\n[REDACTED:TOKEN]"},
	{"short values pass", []Secret{{"PIN", []byte("12345")}}, "pin 12345", "pin 12345"},
	{"exactly min length", []Secret{{"PIN", []byte("123456")}}, "pin 123456", "pin [REDACTED:PIN]"},
	{"prefix value, longest wins", []Secret{{"SHORT", []byte("password")}, {"LONG", []byte("password123")}},
		"password123 password12 password", "[REDACTED:LONG] [REDACTED:SHORT]12 [REDACTED:SHORT]"},
	{"overlapping values merge", []Secret{{"A", []byte("abcdef12")}, {"B", []byte("ef123456")}},
		"xabcdef123456y", "x[REDACTED:A]y"},
	{"overlap chain", []Secret{{"A", []byte("aaaa-1")}, {"B", []byte("-1bbbb-2")}, {"C", []byte("-2cccc")}},
		"aaaa-1bbbb-2cccc!", "[REDACTED:A]!"},
	{"self overlap", []Secret{{"R", []byte("abcabc")}}, "abcabcabc.", "[REDACTED:R]."},
	{"same value, first name", []Secret{{"B", []byte("shared-value")}, {"A", []byte("shared-value")}},
		"shared-value", "[REDACTED:A]"},
	{"partial at end is output", []Secret{{"TOKEN", []byte("s3cr3t-token")}}, "x s3cr3t-tok", "x s3cr3t-tok"},
	{"near miss", []Secret{{"TOKEN", []byte("s3cr3t-token")}}, "s3cr3t-tokem s3cr3t-token", "s3cr3t-tokem [REDACTED:TOKEN]"},
	{"binary", []Secret{{"BIN", []byte{0xff, 0x01, 0xfe, '\n', 0x80, 0x7f}}},
		"\x00\xff\x01\xfe\n\x80\x7f\x00\xff\x01\xfe\n\x80", "\x00[REDACTED:BIN]\x00\xff\x01\xfe\n\x80"},
	{"newline in value", []Secret{{"PEM", []byte("line1\nline2")}}, "a\nline1\nline2\nb", "a\n[REDACTED:PEM]\nb"},
}

func redactAll(secrets []Secret, chunks [][]byte) (string, error) {
	var out bytes.Buffer
	w := NewWriter(&out, secrets)
	for _, c := range chunks {
		if _, err := w.Write(c); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func TestRedact(t *testing.T) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := redactAll(tc.secrets, [][]byte{[]byte(tc.in)})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// TestChunkBoundaries feeds every input split at every pair of points, and
// a byte at a time, so values straddle write boundaries every possible way.
func TestChunkBoundaries(t *testing.T) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			for i := 0; i <= len(in); i++ {
				for j := i; j <= len(in); j++ {
					got, err := redactAll(tc.secrets, [][]byte{in[:i], in[i:j], in[j:]})
					if err != nil || got != tc.want {
						t.Fatalf("split %d,%d: got %q, %v; want %q", i, j, got, err, tc.want)
					}
				}
			}
			var bytewise [][]byte
			for i := range in {
				bytewise = append(bytewise, in[i:i+1])
			}
			if got, err := redactAll(tc.secrets, bytewise); err != nil || got != tc.want {
				t.Fatalf("byte at a time: got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// TestHoldsBackOnlyPrefixes checks that output which cannot start a value
// is written at once, a possible value prefix waits, and Close flushes it.
func TestHoldsBackOnlyPrefixes(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, []Secret{{"TOKEN", []byte("s3cr3t-token")}})
	write := func(s string) {
		t.Helper()
		if n, err := w.Write([]byte(s)); n != len(s) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	write("ready\n")
	if out.String() != "ready\n" {
		t.Fatalf("after plain line: %q", out.String())
	}
	write("x s3cr3t-to")
	if out.String() != "ready\nx " {
		t.Fatalf("after prefix: %q", out.String())
	}
	write("ken y s3c")
	if out.String() != "ready\nx [REDACTED:TOKEN] y " {
		t.Fatalf("after completion: %q", out.String())
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ready\nx [REDACTED:TOKEN] y s3c" {
		t.Fatalf("after Close: %q", out.String())
	}
	if _, err := w.Write([]byte("more")); err == nil {
		t.Fatal("Write after Close succeeded")
	}
}

func TestCopiesValues(t *testing.T) {
	value := []byte("s3cr3t-token")
	var out bytes.Buffer
	w := NewWriter(&out, []Secret{{"TOKEN", value}})
	clear(value)
	w.Write([]byte("s3cr3t-token"))
	w.Close()
	if out.String() != "[REDACTED:TOKEN]" {
		t.Fatalf("got %q", out.String())
	}
}

// TestRandom checks, over random binary streams full of values, partial
// values, and overlaps, that random chunking never changes the output and
// that no value survives.
func TestRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for round := range 300 {
		var secrets []Secret
		for i := range 1 + rng.IntN(5) {
			v := make([]byte, MinLen+rng.IntN(10))
			for j := range v {
				// A small alphabet makes overlaps and near misses common.
				v[j] = "ab\x00\xff\n"[rng.IntN(5)]
			}
			secrets = append(secrets, Secret{Name: string(rune('A' + i)), Value: v})
		}
		var in []byte
		for len(in) < 400 {
			s := secrets[rng.IntN(len(secrets))].Value
			switch rng.IntN(3) {
			case 0:
				in = append(in, s...)
			case 1:
				in = append(in, s[:rng.IntN(len(s))]...)
			default:
				in = append(in, "ab\x00\xff\nxyz"[rng.IntN(8)])
			}
		}

		want, err := redactAll(secrets, [][]byte{in})
		if err != nil {
			t.Fatal(err)
		}
		plain := want
		for _, s := range secrets {
			plain = strings.ReplaceAll(plain, "[REDACTED:"+s.Name+"]", "\x01")
		}
		for _, s := range secrets {
			if strings.Contains(plain, string(s.Value)) {
				t.Fatalf("round %d: %q survives in %q", round, s.Value, want)
			}
		}

		var chunks [][]byte
		for rest := in; len(rest) > 0; {
			n := min(1+rng.IntN(20), len(rest))
			chunks, rest = append(chunks, rest[:n]), rest[n:]
		}
		if got, err := redactAll(secrets, chunks); err != nil || got != want {
			t.Fatalf("round %d: chunked %q, %v; whole %q", round, got, err, want)
		}
	}
}

func FuzzRedact(f *testing.F) {
	f.Add([]byte("s3cr3t-token"), []byte("password"), []byte("x s3cr3t-token password123"), uint8(3))
	f.Add([]byte("abcabc"), []byte("cabcab"), []byte("abcabcabcabc"), uint8(1))
	f.Fuzz(func(t *testing.T, a, b, in []byte, chunk uint8) {
		secrets := []Secret{{"A", a}, {"B", b}}
		want, err := redactAll(secrets, [][]byte{in})
		if err != nil {
			t.Fatal(err)
		}
		n := int(chunk) + 1
		var chunks [][]byte
		for rest := in; len(rest) > 0; {
			k := min(n, len(rest))
			chunks, rest = append(chunks, rest[:k]), rest[k:]
		}
		if got, _ := redactAll(secrets, chunks); got != want {
			t.Fatalf("chunked %q, whole %q", got, want)
		}
	})
}
