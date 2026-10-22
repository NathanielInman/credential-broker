package envfile

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want []Assignment
	}{
		{"export", "export A=plain\n", []Assignment{{Name: "A", Value: "plain", First: 1, Last: 1}}},
		{"bare", "A=plain", []Assignment{{Name: "A", Value: "plain", First: 1, Last: 1}}},
		{"blanks", "  export \t A=x  \n", []Assignment{{Name: "A", Value: "x", First: 1, Last: 1}}},
		{"lower case name", "a_1=x\n", []Assignment{{Name: "a_1", Value: "x", First: 1, Last: 1}}},
		{"empty", "A=\nB=''\n", []Assignment{{Name: "A", First: 1, Last: 1}, {Name: "B", First: 2, Last: 2}}},
		{"single quotes keep everything", `A='$B "c" \n $(d) ~'`,
			[]Assignment{{Name: "A", Value: `$B "c" \n $(d) ~`, First: 1, Last: 1}}},
		{"double quote escapes", `A="q\" d\$ b\\ t\` + "`" + ` n\n x\'"`,
			[]Assignment{{Name: "A", Value: `q" d$ b\ t` + "`" + ` n\n x\'`, First: 1, Last: 1}}},
		{"unquoted escapes", `A=un\ quoted\'s\$`,
			[]Assignment{{Name: "A", Value: `un quoted's$`, First: 1, Last: 1}}},
		{"quote dance", `A='it'"'"'s'`, []Assignment{{Name: "A", Value: "it's", First: 1, Last: 1}}},
		{"concatenated", `A=a'b c'"d e"f`, []Assignment{{Name: "A", Value: "ab cd ef", First: 1, Last: 1}}},
		{"json in single quotes", `export A='{"k":"v","n":{"x":[1,2]}}'`,
			[]Assignment{{Name: "A", Value: `{"k":"v","n":{"x":[1,2]}}`, First: 1, Last: 1}}},
		{"trailing comment", "A=val # it's a comment\nB=2", []Assignment{
			{Name: "A", Value: "val", First: 1, Last: 1}, {Name: "B", Value: "2", First: 2, Last: 2}}},
		{"hash inside word", "A=val#hash", []Assignment{{Name: "A", Value: "val#hash", First: 1, Last: 1}}},
		{"semicolon", "A='val' ; # done\n", []Assignment{{Name: "A", Value: "val", First: 1, Last: 1}}},
		{"literal dollar", `A="cost$" B`, []Assignment{{Name: "A", Reason: Unsupported, First: 1, Last: 1}}},
		{"literal dollars", `A="a$ b$"x$`, []Assignment{{Name: "A", Value: "a$ b$x$", First: 1, Last: 1}}},
		{"dollar at end", `A="a$"`, []Assignment{{Name: "A", Value: "a$", First: 1, Last: 1}}},
		{"multi-line single quotes", "A='line1\nline2'\nB=x\n", []Assignment{
			{Name: "A", Value: "line1\nline2", First: 1, Last: 2}, {Name: "B", Value: "x", First: 3, Last: 3}}},
		{"multi-line double quotes", "\n\nexport PEM=\"-----BEGIN-----\nabc\n-----END-----\"\n", []Assignment{
			{Name: "PEM", Value: "-----BEGIN-----\nabc\n-----END-----", First: 3, Last: 5}}},
		{"continuation", "A=abc\\\ndef\nB=\"x\\\ny\"", []Assignment{
			{Name: "A", Value: "abcdef", First: 1, Last: 2}, {Name: "B", Value: "xy", First: 3, Last: 4}}},
		{"crlf", "A=x\r\nB='y'\r\n", []Assignment{
			{Name: "A", Value: "x", First: 1, Last: 1}, {Name: "B", Value: "y", First: 2, Last: 2}}},
		{"bom", "\uFEFFA=x", []Assignment{{Name: "A", Value: "x", First: 1, Last: 1}}},

		{"variable", "A=$HOME/x", []Assignment{{Name: "A", Reason: VariableReference, First: 1, Last: 1}}},
		{"braced variable", `A="${TOKEN}"`, []Assignment{{Name: "A", Reason: VariableReference, First: 1, Last: 1}}},
		{"default value", `A=${B:-'x y'}`, []Assignment{{Name: "A", Reason: VariableReference, First: 1, Last: 1}}},
		{"positional", `A="x$1"`, []Assignment{{Name: "A", Reason: VariableReference, First: 1, Last: 1}}},
		{"command substitution", "A=$(pass show x)", []Assignment{{Name: "A", Reason: CommandSubstitution, First: 1, Last: 1}}},
		{"backticks", "A=`cat f`\nB=1", []Assignment{
			{Name: "A", Reason: CommandSubstitution, First: 1, Last: 1}, {Name: "B", Value: "1", First: 2, Last: 2}}},
		{"multi-line substitution", "A=\"$(printf '%s' \"a)\n\"\n)\"\nB=1\n", []Assignment{
			{Name: "A", Reason: CommandSubstitution, First: 1, Last: 3}, {Name: "B", Value: "1", First: 4, Last: 4}}},
		{"arithmetic", "A=$((1 + 2))", []Assignment{{Name: "A", Reason: Arithmetic, First: 1, Last: 1}}},
		{"ansi quoting", `A=$'a\'b'`, []Assignment{{Name: "A", Reason: ANSIQuote, First: 1, Last: 1}}},
		{"tilde", "A=~/x", []Assignment{{Name: "A", Reason: TildeExpansion, First: 1, Last: 1}}},
		{"tilde after colon", "A=/a:~/b", []Assignment{{Name: "A", Reason: TildeExpansion, First: 1, Last: 1}}},
		{"quoted tilde", "A='~/x'", []Assignment{{Name: "A", Value: "~/x", First: 1, Last: 1}}},
		{"tilde inside", "A=a~b", []Assignment{{Name: "A", Value: "a~b", First: 1, Last: 1}}},
		{"append", "A+=x", []Assignment{{Name: "A", Reason: Append, First: 1, Last: 1}}},
		{"array", "A=(a b)", []Assignment{{Name: "A", Reason: Unsupported, First: 1, Last: 1}}},
		{"two assignments", "export A=1 B=2", []Assignment{{Name: "A", Reason: Unsupported, First: 1, Last: 1}}},
		{"and list", "A=1&&B=2", []Assignment{{Name: "A", Reason: Unsupported, First: 1, Last: 1}}},
		{"unterminated", "A=ok\nB=\"never closed\nC=1\n", []Assignment{
			{Name: "A", Value: "ok", First: 1, Last: 1}, {Name: "B", Reason: Unterminated, First: 2, Last: 3}}},

		{"export without value", "export A\nexport B C\n", nil},
		{"comments", "# A=1\n  # export B=2\n", nil},
		{"commands", "echo \"export A=1\"\nsource ~/.secrets\n", nil},
		{"quote in a command spans lines", "alias x='\nA=1\n'\nB=2", []Assignment{{Name: "B", Value: "2", First: 4, Last: 4}}},
		{"quote in a comment", "echo hi # don't\nA=1", []Assignment{{Name: "A", Value: "1", First: 2, Last: 2}}},
		{"hash in a command word", "echo a#'\nA=1\n'", nil},
		{"export as a name", "export=1", []Assignment{{Name: "export", Value: "1", First: 1, Last: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.src).Assignments
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q)\n got %+v\nwant %+v", tc.src, got, tc.want)
			}
		})
	}
}

func TestRefs(t *testing.T) {
	src := "export TOKEN=abc\n" +
		"# uses $COMMENTED\n" +
		"export URL=\"https://$USER:${TOKEN}@host\"\n" +
		"LIT='$NOT_A_REF'\n" +
		"[ -n \"$FLAG\" ] && echo \\\n  $TOKEN\n" +
		"X=1 # $IN_COMMENT_AFTER_VALUE\n"
	want := map[string][]int{
		"USER":  {3},
		"TOKEN": {3, 6},
		"FLAG":  {5},
	}
	if got := Parse(src).Refs; !reflect.DeepEqual(got, want) {
		t.Fatalf("Refs = %v, want %v", got, want)
	}
}
