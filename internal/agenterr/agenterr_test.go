package agenterr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
		code int
	}{
		{
			name: "no grant",
			err:  &Error{Code: NoGrant, Names: []string{"X"}, AskHuman: "broker grant X --for 1h"},
			want: `{"error":"no_grant","names":["X"],"ask_human":"broker grant X --for 1h"}` + "\n",
			code: 77,
		},
		{
			name: "unknown secret",
			err:  &Error{Code: UnknownSecret, Names: []string{"A", "B"}},
			want: `{"error":"unknown_secret","names":["A","B"]}` + "\n",
			code: 78,
		},
		{
			name: "wrapped",
			err:  fmt.Errorf("exec: %w", New(Usage, "need a command after --")),
			want: `{"error":"usage","message":"need a command after --"}` + "\n",
			code: 64,
		},
		{
			name: "plain error",
			err:  errors.New("disk on fire & <smoke>"),
			want: `{"error":"error","message":"disk on fire & <smoke>"}` + "\n",
			code: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			code := Write(&buf, tt.err)
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if code != tt.code {
				t.Errorf("exit %d, want %d", code, tt.code)
			}
		})
	}
}

func TestEveryCodeHasExitStatus(t *testing.T) {
	for _, c := range []Code{General, Usage, Unsupported, NotImplemented, NeedsHuman, NoGrant, UnknownSecret, NoVault, VaultExists} {
		if _, ok := exitCodes[c]; !ok {
			t.Errorf("%s has no exit status", c)
		}
	}
}

func TestRequireTTYPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	err = RequireTTY(r, "broker get X")
	e := From(err)
	if err == nil || e.Code != NeedsHuman || e.AskHuman != "broker get X" {
		t.Fatalf("got %v, want needs_human with ask_human", err)
	}
	if e.ExitCode() != 75 {
		t.Fatalf("exit %d, want 75", e.ExitCode())
	}
}

func TestRequireTTYReader(t *testing.T) {
	if e := From(RequireTTY(bytes.NewBufferString("x"), "broker set X")); e.Code != NeedsHuman {
		t.Fatalf("got %v, want needs_human", e)
	}
}
