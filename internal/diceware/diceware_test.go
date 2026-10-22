package diceware

import (
	"strings"
	"testing"
)

func TestWordlist(t *testing.T) {
	if len(Words) != Size {
		t.Fatalf("%d words", len(Words))
	}
	seen := map[string]bool{}
	for _, w := range Words {
		if seen[w] {
			t.Errorf("duplicate word %q", w)
		}
		seen[w] = true
		if strings.ContainsAny(w, " \t\r") {
			t.Errorf("word %q holds whitespace", w)
		}
	}
	if Words[0] != "abacus" || Words[Size-1] != "zoom" {
		t.Errorf("first %q last %q", Words[0], Words[Size-1])
	}
}

func TestPassphrase(t *testing.T) {
	p, err := Passphrase(7)
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Split(p, " ")
	if len(words) != 7 {
		t.Fatalf("%q has %d words", p, len(words))
	}
	known := map[string]bool{}
	for _, w := range Words {
		known[w] = true
	}
	for _, w := range words {
		if !known[w] {
			t.Errorf("%q is not in the wordlist", w)
		}
	}
	if q, _ := Passphrase(7); q == p {
		t.Errorf("two passphrases are both %q", p)
	}
}
