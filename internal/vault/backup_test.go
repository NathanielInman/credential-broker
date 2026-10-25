package vault

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

const testPassphrase = "correct horse battery staple"

func init() {
	// age's default scrypt cost takes about a second per call.
	scryptWorkFactor = 10
}

// seeded is initPlain with private TOKEN and public USER, a line in the
// audit log, and its unlock identity file.
func seeded(t *testing.T) (*Vault, string) {
	t.Helper()
	file, _ := plainIdentity(t)
	v, err := Init(testPaths(t), []string{file}, InitOptions{AllowPlain: true, UI: testUI})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("TOKEN", []byte("token-value"), SetOptions{Description: ptr("deploy")}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Set("USER", []byte("nate"), SetOptions{Tier: Public}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.Paths().LogFile(), []byte(`{"action":"init"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return v, file
}

func backup(t *testing.T, v *Vault, opts BackupOptions) []byte {
	t.Helper()
	priv, err := v.UnlockPrivate(testUI)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if _, err := v.Backup(&b, testPassphrase, priv, opts); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func restore(p paths.Paths, data []byte, opts RestoreOptions) (*Restored, error) {
	opts.UI = testUI
	return Restore(p, bytes.NewReader(data), func() (string, error) { return testPassphrase, nil }, opts)
}

// values decrypts every secret in v, unlocking with its own unlock
// identities.
func values(t *testing.T, v *Vault) map[string]string {
	t.Helper()
	priv, err := v.UnlockPrivate(testUI)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := v.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, s := range secrets {
		value, err := v.Decrypt(s.Name, priv)
		if err != nil {
			t.Fatal(err)
		}
		out[s.Name+"/"+string(s.Tier)+"/"+s.Description] = string(value)
	}
	return out
}

func TestBackupRestore(t *testing.T) {
	v, _ := seeded(t)
	want := values(t, v)
	data := backup(t, v, BackupOptions{IncludeLog: true, Version: "test"})

	// A fresh home and a different unlock identity.
	file, newID := plainIdentity(t)
	p := testPaths(t)
	res, err := restore(p, data, RestoreOptions{IdentityFiles: []string{file}, AllowPlain: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Replaced || res.Manifest.Format != BackupFormat || res.Manifest.Version != "test" || !res.Manifest.Log {
		t.Errorf("restored %+v", res)
	}
	got := values(t, res.Vault)
	if len(got) != 2 || got["TOKEN/private/deploy"] != "token-value" || got["USER/public/"] != "nate" {
		t.Fatalf("restored values %v, want %v", got, want)
	}
	if u := res.Vault.Config().Unlock; len(u) != 1 || u[0].Recipient != newID.Recipient().String() {
		t.Errorf("unlock %+v", u)
	}
	// The backup's log becomes the log of a home that has none.
	if res.Log != p.LogFile() {
		t.Errorf("log went to %q", res.Log)
	}
	if log, _ := os.ReadFile(p.LogFile()); string(log) != `{"action":"init"}`+"\n" {
		t.Errorf("log %q", log)
	}
	// The reopened vault matches.
	v2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := values(t, v2); len(got) != 2 {
		t.Errorf("reopened values %v", got)
	}
	assertNoStaging(t, p)
}

func TestRestoreRequiresPlainFlag(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{})
	file, _ := plainIdentity(t)
	_, err := restore(testPaths(t), data, RestoreOptions{IdentityFiles: []string{file}})
	if code(err) != agenterr.Usage {
		t.Fatalf("got %v", err)
	}
}

// TestBackupStockAge reads a backup the way `age -d FILE | tar -x` does,
// then decrypts each secret with the identity files it holds.
func TestBackupStockAge(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{})
	id, err := age.NewScryptIdentity(testPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	r, err := age.Decrypt(bytes.NewReader(data), id)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(r)
	files := map[string][]byte{}
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		wantMode := int64(paths.FileMode)
		if h.Typeflag == tar.TypeDir {
			wantMode = int64(paths.DirMode)
		}
		if h.Mode != wantMode {
			t.Errorf("%s mode %o", h.Name, h.Mode)
		}
		files[h.Name], _ = io.ReadAll(tr)
	}
	if names[0] != manifestEntry {
		t.Errorf("first entry %s", names[0])
	}
	if _, ok := files[logEntry]; ok {
		t.Error("log included without IncludeLog")
	}
	var m Manifest
	if err := json.Unmarshal(files[manifestEntry], &m); err != nil || m.Format != 1 || m.Log {
		t.Errorf("manifest %s: %v", files[manifestEntry], err)
	}
	for _, tc := range []struct{ key, secret, want string }{
		{privateEntry, "data/secrets/private/TOKEN.age", "token-value"},
		{publicEntry, "data/secrets/public/USER.age", "nate"},
	} {
		ids, err := age.ParseIdentities(bytes.NewReader(files[tc.key]))
		if err != nil {
			t.Fatal(err)
		}
		r, err := age.Decrypt(bytes.NewReader(files[tc.secret]), ids...)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := io.ReadAll(r); string(got) != tc.want {
			t.Errorf("%s = %q", tc.secret, got)
		}
	}
	if !bytes.Contains(files["data/secrets/private/TOKEN.json"], []byte(`"deploy"`)) {
		t.Errorf("metadata %s", files["data/secrets/private/TOKEN.json"])
	}
}

func TestRestoreRefusesExistingVault(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{})

	// A second vault with its own secret and log.
	other, otherFile := seeded(t)
	if _, err := other.Set("OTHER", []byte("other-value"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	p := other.Paths()
	// It fails before asking for the passphrase.
	_, err := Restore(p, bytes.NewReader(data), func() (string, error) {
		t.Error("prompted for a passphrase")
		return testPassphrase, nil
	}, RestoreOptions{UI: testUI})
	if code(err) != agenterr.VaultExists {
		t.Fatalf("got %v, want vault_exists", err)
	}
	if got := values(t, other); len(got) != 3 {
		t.Fatalf("refused restore changed the vault: %v", got)
	}

	// --force replaces it, wrapped to the backup's own unlock identity, so
	// the other vault's identity no longer opens it.
	res, err := restore(p, data, RestoreOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Replaced || res.Log != "" {
		t.Errorf("restored %+v", res)
	}
	if got := values(t, res.Vault); len(got) != 2 || got["TOKEN/private/deploy"] != "token-value" {
		t.Fatalf("values %v", got)
	}
	if _, err := os.Stat(filepath.Join(p.TierDir("private"), "OTHER.age")); !os.IsNotExist(err) {
		t.Errorf("old secret survived the restore: %v", err)
	}
	_, ids, err := readIdentityFile(otherFile, testUI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptFile(p.PrivateKey(), ids...); err == nil {
		t.Error("the replaced vault's identity still unwraps the private key")
	}
	assertNoStaging(t, p)
}

func TestRestoreKeepsLocalLog(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{IncludeLog: true})
	if err := os.WriteFile(v.Paths().LogFile(), []byte(`{"action":"set"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := restore(v.Paths(), data, RestoreOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if log, _ := os.ReadFile(v.Paths().LogFile()); string(log) != `{"action":"set"}`+"\n" {
		t.Errorf("local log rewritten: %q", log)
	}
	if filepath.Dir(res.Log) != v.Paths().State || !strings.HasPrefix(filepath.Base(res.Log), "log.restored-") {
		t.Fatalf("backup log went to %q", res.Log)
	}
	if log, _ := os.ReadFile(res.Log); string(log) != `{"action":"init"}`+"\n" {
		t.Errorf("backup log %q", log)
	}
}

func TestRestoreWrongPassphrase(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{})
	p := testPaths(t)
	_, err := Restore(p, bytes.NewReader(data), func() (string, error) { return "wrong", nil }, RestoreOptions{UI: testUI})
	if err == nil || !strings.Contains(err.Error(), "wrong passphrase") {
		t.Fatalf("got %v", err)
	}
	if Existing(p) != "" {
		t.Error("failed restore left a vault")
	}
	assertNoStaging(t, p)
}

func TestRestoreForgivesSpacing(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{})
	p := testPaths(t)
	typed := "  correct  horse battery\tstaple "
	if _, err := Restore(p, bytes.NewReader(data), func() (string, error) { return typed, nil }, RestoreOptions{UI: testUI}); err != nil {
		t.Fatal(err)
	}
	if Existing(p) == "" {
		t.Error("restore left no vault")
	}
}

// TestRestoreBadBackups checks that a backup that fails any check leaves
// the existing vault exactly as it was.
func TestRestoreBadBackups(t *testing.T) {
	v, _ := seeded(t)
	data := backup(t, v, BackupOptions{IncludeLog: true})
	for _, tc := range []struct {
		name string
		edit func(name string, body []byte) (string, []byte)
		want string
	}{
		{"newer format", func(name string, body []byte) (string, []byte) {
			if name == manifestEntry {
				body = []byte(`{"format":2}`)
			}
			return name, body
		}, "newer than this broker"},
		{"no manifest", func(name string, body []byte) (string, []byte) {
			if name == manifestEntry {
				name = "other.json"
			}
			return name, body
		}, "no manifest"},
		{"traversal", func(name string, body []byte) (string, []byte) {
			if name == "data/secrets/public/USER.age" {
				name = "data/secrets/public/../../../evil"
			}
			return name, body
		}, "unexpected entry"},
		{"bad name", func(name string, body []byte) (string, []byte) {
			if name == "data/secrets/public/USER.age" {
				name = "data/secrets/public/user.age"
			}
			return name, body
		}, "unexpected entry"},
		{"corrupt secret", func(name string, body []byte) (string, []byte) {
			if name == "data/secrets/private/TOKEN.age" {
				body = bytes.Clone(body)
				body[len(body)-1] ^= 1
			}
			return name, body
		}, "does not decrypt"},
		{"missing key", func(name string, body []byte) (string, []byte) {
			if name == privateEntry {
				return "", nil
			}
			return name, body
		}, "missing data/private.key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other, _ := seeded(t)
			before := values(t, other)
			_, err := restore(other.Paths(), rewrite(t, data, tc.edit), RestoreOptions{Force: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			reopened, err := Open(other.Paths())
			if err != nil {
				t.Fatal(err)
			}
			after := values(t, reopened)
			if len(after) != len(before) {
				t.Fatalf("vault changed: %v -> %v", before, after)
			}
			for k, v := range before {
				if after[k] != v {
					t.Errorf("%s changed", k)
				}
			}
			assertNoStaging(t, other.Paths())
			entries, _ := os.ReadDir(other.Paths().State)
			if len(entries) != 1 {
				t.Errorf("state holds %v", entries)
			}
		})
	}
}

// rewrite decrypts a backup, passes each entry through edit (an empty name
// drops it), and encrypts the result again.
func rewrite(t *testing.T, data []byte, edit func(name string, body []byte) (string, []byte)) []byte {
	t.Helper()
	id, _ := age.NewScryptIdentity(testPassphrase)
	r, err := age.Decrypt(bytes.NewReader(data), id)
	if err != nil {
		t.Fatal(err)
	}
	rcpt, _ := age.NewScryptRecipient(testPassphrase)
	rcpt.SetWorkFactor(scryptWorkFactor)
	var out bytes.Buffer
	w, err := age.Encrypt(&out, rcpt)
	if err != nil {
		t.Fatal(err)
	}
	tr, tw := tar.NewReader(r), tar.NewWriter(w)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(tr)
		if h.Name, body = edit(h.Name, body); h.Name == "" {
			continue
		}
		h.Size = int64(len(body))
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write(body)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// assertNoStaging fails if a restore left its staging directory behind.
func assertNoStaging(t *testing.T, p paths.Paths) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(p.Data), ".broker-restore-*"))
	if len(left) > 0 {
		t.Errorf("staging left behind: %v", left)
	}
}
