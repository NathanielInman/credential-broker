package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := &Config{
		DefaultGrant:     DefaultGrant,
		PublicRecipient:  "age1pub",
		PrivateRecipient: "age1priv",
		Unlock: []Unlock{
			{Path: "/k/yubikey.txt", Identity: "AGE-PLUGIN-YUBIKEY-1XYZ"},
			{Path: "/k/plain.txt", Recipient: "age1plain"},
		},
	}
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.Unlock[0].Plain() || !got.Unlock[1].Plain() {
		t.Error("Plain() is wrong")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want ErrNotExist", err)
	}
}
