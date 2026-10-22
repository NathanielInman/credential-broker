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
			{Path: "/k/pass.age", Recipient: "age1pass", Passphrase: true},
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
	for i, want := range []Zone{ZoneHardware, ZonePlain, ZonePassphrase} {
		if z := got.Unlock[i].Zone(); z != want {
			t.Errorf("unlock %d: zone %s, want %s", i, z, want)
		}
	}
}

func TestConfigZoneIsWeakest(t *testing.T) {
	hw := Unlock{Path: "/k/y.txt", Identity: "AGE-PLUGIN-YUBIKEY-1XYZ"}
	pass := Unlock{Path: "/k/p.age", Recipient: "age1p", Passphrase: true}
	plain := Unlock{Path: "/k/k.txt", Recipient: "age1k"}
	for _, tc := range []struct {
		unlocks []Unlock
		want    Zone
	}{
		{[]Unlock{hw}, ZoneHardware},
		{[]Unlock{hw, pass}, ZonePassphrase},
		{[]Unlock{pass, hw, plain}, ZonePlain},
	} {
		if z := (&Config{Unlock: tc.unlocks}).Zone(); z != tc.want {
			t.Errorf("%+v: zone %s, want %s", tc.unlocks, z, tc.want)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want ErrNotExist", err)
	}
}

func TestUnlockPlugin(t *testing.T) {
	for stub, want := range map[string]string{
		"AGE-PLUGIN-YUBIKEY-1QYQSZQGPQYQSZQGP": "yubikey",
		"AGE-PLUGIN-SE-1QJPQZLE3SGQHKVYP75X6K": "se",
		"AGE-PLUGIN-FIDO2-HMAC-1QQPQZQGPQYQS":  "fido2-hmac",
		"":                                     "",
	} {
		if got := (Unlock{Identity: stub}).Plugin(); got != want {
			t.Errorf("Plugin(%q) = %q, want %q", stub, got, want)
		}
	}
}
