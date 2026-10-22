package vault

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"filippo.io/age/plugin"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// testPlugin is a fake age plugin served by this test binary: its identity
// data is a plain X25519 identity string. It exercises the real plugin
// client protocol without hardware.
const testPlugin = "brokertest"

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "age-plugin-"+testPlugin {
		os.Exit(runTestPlugin())
	}
	os.Exit(m.Run())
}

func runTestPlugin() int {
	p, err := plugin.New(testPlugin)
	if err != nil {
		panic(err)
	}
	fs := flag.NewFlagSet(testPlugin, flag.ExitOnError)
	p.RegisterFlags(fs)
	_ = fs.Parse(os.Args[1:])
	p.HandleIdentity(func(data []byte) (age.Identity, error) {
		return age.ParseX25519Identity(string(data))
	})
	p.HandleIdentityAsRecipient(func(data []byte) (age.Recipient, error) {
		id, err := age.ParseX25519Identity(string(data))
		if err != nil {
			return nil, err
		}
		return id.Recipient(), nil
	})
	return p.Main()
}

// installTestPlugin puts age-plugin-brokertest (this binary) on PATH.
func installTestPlugin(t *testing.T) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(dir, "age-plugin-"+testPlugin)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("BROKER_HOME", t.TempDir())
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeIdentity(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "identity.txt")
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func plainIdentity(t *testing.T) (string, *age.X25519Identity) {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return writeIdentity(t, "# test key\n"+id.String()+"\n"), id
}

var testUI = TerminalUI(io.Discard)

func initPlain(t *testing.T) *Vault {
	t.Helper()
	file, _ := plainIdentity(t)
	v, err := Init(testPaths(t), []string{file}, InitOptions{AllowPlain: true, UI: testUI})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func code(err error) agenterr.Code {
	if err == nil {
		return ""
	}
	return agenterr.From(err).Code
}

func ptr[T any](v T) *T { return &v }

func TestInitRefusesPlainWithoutFlag(t *testing.T) {
	file, _ := plainIdentity(t)
	p := testPaths(t)
	_, err := Init(p, []string{file}, InitOptions{UI: testUI})
	if code(err) != agenterr.Usage {
		t.Fatalf("got %v, want usage error", err)
	}
	if _, err := os.Stat(p.Data); !errors.Is(err, fs.ErrNotExist) {
		t.Error("refused init still created the data directory")
	}
}

func TestInitRefusesExistingVault(t *testing.T) {
	v := initPlain(t)
	file, _ := plainIdentity(t)
	_, err := Init(v.Paths(), []string{file}, InitOptions{AllowPlain: true, UI: testUI})
	if code(err) != agenterr.VaultExists {
		t.Fatalf("got %v, want vault_exists", err)
	}
}

func TestInitBadIdentityFile(t *testing.T) {
	for name, content := range map[string]string{
		"empty":   "# nothing here\n",
		"garbage": "not a key\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Init(testPaths(t), []string{writeIdentity(t, content)}, InitOptions{AllowPlain: true, UI: testUI})
			if err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestOpenWithoutVault(t *testing.T) {
	_, err := Open(testPaths(t))
	e := agenterr.From(err)
	if e.Code != agenterr.NoVault || e.AskHuman == "" {
		t.Fatalf("got %v, want no_vault with ask_human", err)
	}
}

func TestRoundTrip(t *testing.T) {
	v := initPlain(t)
	if _, err := v.Set("API_TOKEN", []byte("s3cret-token"), SetOptions{Description: ptr("deploy token")}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("API_USER", []byte("nate"), SetOptions{Tier: Public}); err != nil {
		t.Fatal(err)
	}

	v, err := Open(v.Paths())
	if err != nil {
		t.Fatal(err)
	}
	list, err := v.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 ||
		list[0].Name != "API_TOKEN" || list[0].Tier != Private || list[0].Description != "deploy token" ||
		list[1].Name != "API_USER" || list[1].Tier != Public {
		t.Fatalf("List = %+v", list)
	}
	if list[0].Created.IsZero() || list[0].Updated.IsZero() {
		t.Errorf("timestamps not set: %+v", list[0])
	}

	if got, err := v.Decrypt("API_USER", nil); err != nil || string(got) != "nate" {
		t.Fatalf("public Decrypt = %q, %v", got, err)
	}
	if _, err := v.Decrypt("API_TOKEN", nil); err == nil {
		t.Fatal("private Decrypt without identity succeeded")
	}
	priv, err := v.UnlockPrivate(testUI)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := v.Decrypt("API_TOKEN", priv); err != nil || string(got) != "s3cret-token" {
		t.Fatalf("private Decrypt = %q, %v", got, err)
	}

	if err := v.Remove("API_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get("API_TOKEN"); code(err) != agenterr.UnknownSecret {
		t.Fatalf("Get after Remove: %v", err)
	}
	if err := v.Remove("API_TOKEN"); code(err) != agenterr.UnknownSecret {
		t.Fatalf("second Remove: %v", err)
	}
	if list, _ := v.List(); len(list) != 1 {
		t.Fatalf("List after Remove = %+v", list)
	}
}

func TestPublicIdentityCannotReadPrivate(t *testing.T) {
	v := initPlain(t)
	if _, err := v.Set("X_TOKEN", []byte("private value"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	pub, err := v.PublicIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Decrypt("X_TOKEN", pub); err == nil {
		t.Fatal("public identity decrypted a private secret")
	}
	var noMatch *age.NoIdentityMatchError
	if _, err := decryptFile(v.valuePath("X_TOKEN", Private), pub); !errors.As(err, &noMatch) {
		t.Fatalf("got %v, want NoIdentityMatchError", err)
	}
	// Nor can the public identity unwrap private.key.age.
	if _, err := decryptFile(v.Paths().PrivateKey(), pub); !errors.As(err, &noMatch) {
		t.Fatalf("got %v, want NoIdentityMatchError", err)
	}
}

func TestOverwriteKeepsTierUnlessAsked(t *testing.T) {
	v := initPlain(t)
	first, err := v.Set("X", []byte("one"), SetOptions{Tier: Public, Description: ptr("d")})
	if err != nil {
		t.Fatal(err)
	}
	s, err := v.Set("X", []byte("two"), SetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Tier != Public || s.Description != "d" || !s.Created.Equal(first.Created) {
		t.Fatalf("plain overwrite changed tier, description or created: %+v", s)
	}
	if got, _ := v.Decrypt("X", nil); string(got) != "two" {
		t.Fatalf("value %q, want two", got)
	}

	if s, err = v.Set("X", []byte("three"), SetOptions{Tier: Private, Description: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if s.Tier != Private || s.Description != "" {
		t.Fatalf("move to private: %+v", s)
	}
	for _, f := range []string{v.valuePath("X", Public), v.metaPath("X", Public)} {
		if _, err := os.Stat(f); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s left behind after move", f)
		}
	}
	if list, _ := v.List(); len(list) != 1 || list[0].Tier != Private {
		t.Fatalf("List after move = %+v", list)
	}
}

func TestBothTiersNeedsExplicitTier(t *testing.T) {
	v := initPlain(t)
	if _, err := v.Set("X", []byte("one"), SetOptions{Tier: Public}); err != nil {
		t.Fatal(err)
	}
	// Simulate an interrupted move: a copy left in the private tier.
	data, _ := os.ReadFile(v.valuePath("X", Public))
	if err := paths.WriteFile(v.valuePath("X", Private), data); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get("X"); err == nil {
		t.Fatal("Get succeeded with X in both tiers")
	}
	if _, err := v.Set("X", []byte("two"), SetOptions{}); err == nil {
		t.Fatal("Set without a tier succeeded with X in both tiers")
	}
	if _, err := v.Set("X", []byte("two"), SetOptions{Tier: Public}); err != nil {
		t.Fatal(err)
	}
	if s, err := v.Get("X"); err != nil || s.Tier != Public {
		t.Fatalf("Get after repair = %+v, %v", s, err)
	}
}

func TestValidation(t *testing.T) {
	v := initPlain(t)
	for _, name := range []string{"", "lower", "1ABC", "A-B", "../X", "X.age", "A B"} {
		if _, err := v.Set(name, []byte("v"), SetOptions{}); code(err) != agenterr.Usage {
			t.Errorf("Set(%q): %v, want usage", name, err)
		}
		if _, err := v.Get(name); code(err) != agenterr.Usage {
			t.Errorf("Get(%q): %v, want usage", name, err)
		}
	}
	for _, name := range []string{"A", "_", "_A1", "AWS_SECRET_ACCESS_KEY"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q): %v", name, err)
		}
	}
	for _, value := range [][]byte{nil, []byte("a\x00b"), bytes.Repeat([]byte("a"), MaxValue+1)} {
		if _, err := v.Set("X", value, SetOptions{}); code(err) != agenterr.Usage {
			t.Errorf("Set(len %d): %v, want usage", len(value), err)
		}
	}
	if _, err := v.Set("X", []byte("v"), SetOptions{Description: ptr("two\nlines")}); code(err) != agenterr.Usage {
		t.Errorf("multi-line description: %v, want usage", err)
	}
}

func TestFileModes(t *testing.T) {
	v := initPlain(t)
	if _, err := v.Set("A", []byte("private"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("B", []byte("public"), SetOptions{Tier: Public}); err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("BROKER_HOME")
	var files int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err // root is the test's temp dir, not broker's
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		want := paths.FileMode
		if d.IsDir() {
			want = paths.DirMode
		} else {
			files++
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s mode %v, want %v", path, fi.Mode().Perm(), want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// config, public.key, private.key.age, and two files per secret.
	if files != 7 {
		t.Errorf("found %d files, want 7", files)
	}
}

func TestStockAgeLayout(t *testing.T) {
	file, unlock := plainIdentity(t)
	v, err := Init(testPaths(t), []string{file}, InitOptions{AllowPlain: true, UI: testUI})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("X", []byte("value"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	// What a human would do with the age CLI: unwrap private.key.age with
	// the unlock identity, then decrypt the secret with the result.
	data, err := decryptFile(v.Paths().PrivateKey(), unlock)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := age.ParseIdentities(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := decryptFile(filepath.Join(v.Paths().TierDir("private"), "X.age"), ids...)
	if err != nil || string(got) != "value" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestPluginIdentity(t *testing.T) {
	installTestPlugin(t)
	hw, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	stub := plugin.EncodeIdentity(testPlugin, []byte(hw.String()))
	file := writeIdentity(t, "# Recipient: (fake hardware)\n"+stub+"\n")

	// No --insecure-plain-identity needed for a plugin identity.
	v, err := Init(testPaths(t), []string{file}, InitOptions{UI: testUI})
	if err != nil {
		t.Fatal(err)
	}
	if u := v.Config().Unlock; len(u) != 1 || u[0].Identity != stub || u[0].Recipient != "" {
		t.Fatalf("unlock entries %+v", u)
	}
	if _, err := v.Set("X", []byte("value"), SetOptions{}); err != nil {
		t.Fatal(err)
	}

	// The stub lives in config, so unlocking works after the file is gone.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	priv, err := v.UnlockPrivate(testUI)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := v.Decrypt("X", priv); err != nil || string(got) != "value" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestPluginMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	stub := plugin.EncodeIdentity("brokermissing", []byte("x"))
	p := testPaths(t)
	_, err := Init(p, []string{writeIdentity(t, stub+"\n")}, InitOptions{UI: testUI})
	var nf *plugin.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("got %v, want plugin.NotFoundError", err)
	}
	if _, err := os.Stat(p.ConfigFile()); !errors.Is(err, fs.ErrNotExist) {
		t.Error("failed init wrote a config")
	}
}

func TestUnlockWithMissingPlainFile(t *testing.T) {
	file, _ := plainIdentity(t)
	v, err := Init(testPaths(t), []string{file}, InitOptions{AllowPlain: true, UI: testUI})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := v.UnlockPrivate(testUI); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got %v, want the missing file reported", err)
	}
}
