// Package diceware generates passphrases from the EFF large wordlist.
package diceware

import (
	"bufio"
	"crypto/rand"
	_ "embed"
	"fmt"
	"math/big"
	"strings"
)

// wordlist is the EFF large wordlist, embedded unchanged: 7776 lines of five
// dice digits, a tab, and a word.
//
// Source: https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt
// (Electronic Frontier Foundation, 2016), licensed CC BY 3.0 US
// (https://www.eff.org/copyright). sha256:
// addd35536511597a02fa0a9ff1e5284677b8883b83e986e43f15a3db996b903e
//
//go:embed eff_large_wordlist.txt
var wordlist string

// Words is the wordlist in dice order.
var Words = parse(wordlist)

// Size is the number of words, 6^5.
const Size = 7776

func parse(list string) []string {
	words := make([]string, 0, Size)
	sc := bufio.NewScanner(strings.NewReader(list))
	for sc.Scan() {
		_, word, ok := strings.Cut(sc.Text(), "\t")
		if !ok || word == "" {
			panic(fmt.Sprintf("diceware: malformed wordlist line %q", sc.Text()))
		}
		words = append(words, word)
	}
	if len(words) != Size {
		panic(fmt.Sprintf("diceware: wordlist has %d words, want %d", len(words), Size))
	}
	return words
}

// Passphrase returns n words chosen uniformly at random with crypto/rand,
// joined by spaces. Each word adds log2(7776), about 12.9 bits.
func Passphrase(n int) (string, error) {
	out := make([]string, n)
	for i := range out {
		j, err := rand.Int(rand.Reader, big.NewInt(Size))
		if err != nil {
			return "", err
		}
		out[i] = Words[j.Int64()]
	}
	return strings.Join(out, " "), nil
}
