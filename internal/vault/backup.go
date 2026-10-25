package vault

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// BackupFormat is the version of the backup layout Backup writes. Restore
// reads every format up to it and refuses newer ones, so a backup made by a
// later broker fails clearly instead of restoring half a vault.
const BackupFormat = 1

// A backup is an age file encrypted to a single scrypt (passphrase)
// recipient, holding an uncompressed tar laid out like BROKER_HOME, so
// `age -d FILE | tar -x` recovers it without broker:
//
//	manifest.json                       always the first entry
//	config/config.json
//	data/public.key
//	data/private.key                    the private tier identity, unwrapped
//	data/secrets/<tier>/<NAME>.age
//	data/secrets/<tier>/<NAME>.json
//	state/log.jsonl                     only with IncludeLog
const (
	manifestEntry = "manifest.json"
	configEntry   = "config/config.json"
	publicEntry   = "data/public.key"
	privateEntry  = "data/private.key"
	logEntry      = "state/log.jsonl"
)

// maxEntry bounds every entry but the log, which streams to disk. A value
// is at most MaxValue, so its age file is far smaller.
const maxEntry = 1 << 20

// scryptWorkFactor overrides age's default scrypt cost (2^18) when set.
// Only tests lower it.
var scryptWorkFactor int

// Manifest is a backup's manifest.json.
type Manifest struct {
	Format  int       `json:"format"`
	Created time.Time `json:"created"`
	// Version is the broker version that wrote the backup.
	Version string `json:"broker_version"`
	// Log reports whether the backup holds the audit log.
	Log bool `json:"log"`
}

// BackupOptions configures Backup.
type BackupOptions struct {
	// IncludeLog adds the audit log.
	IncludeLog bool
	// Version is the broker version recorded in the manifest.
	Version string
}

// Backup writes the vault to w, encrypted to passphrase alone. The private
// tier identity goes in unwrapped, so the backup restores with nothing but
// the passphrase; private is that identity, from UnlockPrivate. It returns
// the secrets written.
func (v *Vault) Backup(w io.Writer, passphrase string, private age.Identity, opts BackupOptions) ([]Secret, error) {
	priv, ok := private.(*age.X25519Identity)
	if !ok || priv.Recipient().String() != v.cfg.PrivateRecipient {
		return nil, errors.New("backup needs the unlocked private tier identity")
	}
	secrets, err := v.List()
	if err != nil {
		return nil, err
	}
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	if scryptWorkFactor != 0 {
		r.SetWorkFactor(scryptWorkFactor)
	}
	aw, err := age.Encrypt(w, r)
	if err != nil {
		return nil, err
	}
	// A vault that has never logged anything has no log to include.
	includeLog := opts.IncludeLog
	if _, err := os.Stat(v.paths.LogFile()); errors.Is(err, os.ErrNotExist) {
		includeLog = false
	}
	now := time.Now().UTC().Truncate(time.Second)
	tw := &tarWriter{tw: tar.NewWriter(aw), now: now}

	m, err := json.MarshalIndent(Manifest{Format: BackupFormat, Created: now, Version: opts.Version, Log: includeLog}, "", "  ")
	if err != nil {
		return nil, err
	}
	tw.file(manifestEntry, append(m, '\n'))
	tw.dir("config")
	tw.copy(configEntry, v.paths.ConfigFile())
	tw.dir("data")
	tw.copy(publicEntry, v.paths.PublicKey())
	privFile := identityFile(priv, Private)
	tw.file(privateEntry, privFile)
	clear(privFile)
	tw.dir("data/secrets")
	for _, t := range Tiers {
		tw.dir("data/secrets/" + string(t))
	}
	for _, s := range secrets {
		dir := "data/secrets/" + string(s.Tier) + "/"
		tw.copy(dir+s.Name+".age", v.valuePath(s.Name, s.Tier))
		// Metadata is optional (see readMeta).
		if _, err := os.Lstat(v.metaPath(s.Name, s.Tier)); err == nil {
			tw.copy(dir+s.Name+".json", v.metaPath(s.Name, s.Tier))
		}
	}
	if includeLog {
		tw.dir("state")
		tw.copy(logEntry, v.paths.LogFile())
	}
	if tw.err != nil {
		return nil, tw.err
	}
	if err := tw.tw.Close(); err != nil {
		return nil, err
	}
	if err := aw.Close(); err != nil {
		return nil, err
	}
	return secrets, nil
}

// tarWriter writes backup entries, keeping the first error.
type tarWriter struct {
	tw  *tar.Writer
	now time.Time
	err error
}

func (w *tarWriter) dir(name string) {
	if w.err == nil {
		w.err = w.tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir, Name: name + "/", Mode: int64(paths.DirMode), ModTime: w.now,
		})
	}
}

func (w *tarWriter) file(name string, data []byte) {
	if w.err != nil {
		return
	}
	if w.err = w.header(name, int64(len(data))); w.err == nil {
		_, w.err = w.tw.Write(data)
	}
}

// copy streams the file at src, so a large log never sits in memory. The
// size is fixed when it starts: lines appended meanwhile are left out.
func (w *tarWriter) copy(name, src string) {
	if w.err != nil {
		return
	}
	f, err := os.Open(src)
	if err != nil {
		w.err = err
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		w.err = err
		return
	}
	if w.err = w.header(name, fi.Size()); w.err == nil {
		_, w.err = io.CopyN(w.tw, f, fi.Size())
	}
}

func (w *tarWriter) header(name string, size int64) error {
	return w.tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg, Name: name, Size: size, Mode: int64(paths.FileMode), ModTime: w.now,
	})
}

// RestoreOptions configures Restore.
type RestoreOptions struct {
	// IdentityFiles are the identities to wrap the private tier to. Empty
	// keeps the unlock identities in the backup's config.
	IdentityFiles []string
	// AllowPlain accepts plain AGE-SECRET-KEY identities in IdentityFiles.
	AllowPlain bool
	// Force replaces an existing vault.
	Force bool
	// UI handles plugin prompts.
	UI *plugin.ClientUI
}

// Restored describes a finished Restore.
type Restored struct {
	Vault    *Vault
	Manifest Manifest
	// Replaced reports whether an existing vault was overwritten.
	Replaced bool
	// Log is where the backup's audit log went, or "" if it had none. The
	// local log is never rewritten: the backup's becomes the log only when
	// there is no local one, and otherwise is kept beside it.
	Log string
}

// Restore rebuilds the vault at p from the backup in r. Everything is
// unpacked, checked (every secret must decrypt), and rewrapped in a staging
// directory first; the live data directory is then swapped for it with one
// rename and config.json written last. A failed restore leaves the old vault
// as it was, and an interrupted one leaves no config.json, which Open
// reports as no vault and a second restore --force repairs. Old and new
// secrets never mix.
//
// passphrase is called once the options check out, so a restore that is
// bound to fail does not prompt first.
func Restore(p paths.Paths, r io.Reader, passphrase func() (string, error), opts RestoreOptions) (*Restored, error) {
	existing := Existing(p)
	if existing != "" && !opts.Force {
		return nil, agenterr.New(agenterr.VaultExists, "a vault already exists (%s); pass --force to replace it", existing)
	}
	var unlocks []config.Unlock
	if len(opts.IdentityFiles) > 0 {
		var err error
		if unlocks, err = loadUnlocks(opts.IdentityFiles, opts.AllowPlain, opts.UI); err != nil {
			return nil, err
		}
	}

	pass, err := passphrase()
	if err != nil {
		return nil, err
	}
	id, err := age.NewScryptIdentity(pass)
	if err != nil {
		return nil, err
	}
	ids := []age.Identity{id}
	// Generated passphrases are words joined by single spaces, so a stray
	// or doubled space is a typo. The passphrase as typed goes first, for
	// one chosen with odd spacing on purpose.
	if words := strings.Join(strings.Fields(pass), " "); words != pass && words != "" {
		wid, err := age.NewScryptIdentity(words)
		if err != nil {
			return nil, err
		}
		ids = append(ids, wid)
	}
	ar, err := age.Decrypt(r, ids...)
	if err != nil {
		var nomatch *age.NoIdentityMatchError
		if errors.As(err, &nomatch) {
			return nil, errors.New("wrong passphrase, or not a broker backup")
		}
		return nil, fmt.Errorf("decrypt backup: %w", err)
	}

	// Stage beside the data directory, so the swap is a rename on one
	// filesystem. The parent is not broker's, so it is created if needed
	// but never re-moded.
	if err := os.MkdirAll(filepath.Dir(p.Data), paths.DirMode); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(p.Data), ".broker-restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := paths.MkdirAll(p.State); err != nil {
		return nil, err
	}
	staged := paths.Paths{Data: filepath.Join(stage, "data")}
	for _, t := range Tiers {
		if err := paths.MkdirAll(staged.TierDir(string(t))); err != nil {
			return nil, err
		}
	}

	u := &unpacked{stage: staged, state: p.State}
	defer u.cleanup()
	if err := u.read(tar.NewReader(ar)); err != nil {
		return nil, err
	}

	var cfg config.Config
	if err := json.Unmarshal(u.config, &cfg); err != nil {
		return nil, fmt.Errorf("backup %s: %w", configEntry, err)
	}
	pub, err := parseTierIdentity(u.public, cfg.PublicRecipient)
	if err != nil {
		return nil, fmt.Errorf("backup %s: %w", publicEntry, err)
	}
	priv, err := parseTierIdentity(u.private, cfg.PrivateRecipient)
	if err != nil {
		return nil, fmt.Errorf("backup %s: %w", privateEntry, err)
	}
	sv := &Vault{paths: staged, cfg: &cfg}
	secrets, err := sv.List()
	if err != nil {
		return nil, err
	}
	for _, s := range secrets {
		var tid age.Identity = priv
		if s.Tier == Public {
			tid = pub
		}
		value, err := decryptFile(sv.valuePath(s.Name, s.Tier), tid)
		if err != nil {
			return nil, fmt.Errorf("backup secret %s does not decrypt: %w", s.Name, err)
		}
		clear(value)
	}

	if unlocks != nil {
		cfg.Unlock = unlocks
	} else if len(cfg.Unlock) == 0 {
		return nil, agenterr.New(agenterr.Usage, "the backup's config has no unlock identities; pass --identity FILE")
	}
	wrapped, err := wrapPrivate(u.private, cfg.Unlock, opts.UI)
	if err != nil {
		return nil, err
	}
	if err := paths.WriteFile(staged.PublicKey(), u.public); err != nil {
		return nil, err
	}
	if err := paths.WriteFile(staged.PrivateKey(), wrapped); err != nil {
		return nil, err
	}

	if err := swap(p, staged.Data, filepath.Join(stage, "old"), &cfg); err != nil {
		return nil, err
	}
	res := &Restored{
		Vault:    &Vault{paths: p, cfg: &cfg},
		Manifest: u.manifest,
		Replaced: existing != "",
	}
	if u.log != "" {
		if res.Log, err = u.placeLog(p.LogFile()); err != nil {
			return res, fmt.Errorf("vault restored, but the backup's audit log was not: %w", err)
		}
	}
	return res, nil
}

// swap replaces p's data directory with staged and writes cfg. The old
// config.json is removed first, so a vault that is mid swap has none and
// never opens with mismatched keys; on failure the old directory and config
// come back.
func swap(p paths.Paths, staged, old string, cfg *config.Config) error {
	oldConfig, err := os.ReadFile(p.ConfigFile())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := paths.MkdirAll(p.Config); err != nil {
		return err
	}
	if err := os.Remove(p.ConfigFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	moved := false
	rollback := func(err error) error {
		if moved {
			_ = os.Rename(p.Data, staged)
			_ = os.Rename(old, p.Data)
		}
		if oldConfig != nil {
			_ = paths.WriteFile(p.ConfigFile(), oldConfig)
		}
		return err
	}
	if err := os.Rename(p.Data, old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return rollback(err)
	}
	moved = true
	if err := os.Rename(staged, p.Data); err != nil {
		return rollback(err)
	}
	if err := cfg.Save(p.ConfigFile()); err != nil {
		return rollback(err)
	}
	return nil
}

// unpacked is a backup read into a staging directory. The private tier
// identity is only ever held in memory.
type unpacked struct {
	stage paths.Paths
	state string

	manifest                Manifest
	config, public, private []byte
	log                     string // staged log file in state, if any
	seen                    map[string]bool
}

func (u *unpacked) read(tr *tar.Reader) error {
	u.seen = map[string]bool{}
	for first := true; ; first = false {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if first && h.Name != manifestEntry {
			return errors.New("not a broker backup: no manifest")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg || path.Clean(h.Name) != h.Name || u.seen[h.Name] {
			return fmt.Errorf("unexpected entry %q in backup", h.Name)
		}
		u.seen[h.Name] = true
		if err := u.entry(h, tr); err != nil {
			return err
		}
	}
	if u.manifest.Format == 0 {
		return errors.New("not a broker backup: no manifest")
	}
	for _, name := range []string{configEntry, publicEntry, privateEntry} {
		if !u.seen[name] {
			return fmt.Errorf("backup is missing %s", name)
		}
	}
	return nil
}

func (u *unpacked) entry(h *tar.Header, r io.Reader) error {
	switch h.Name {
	case manifestEntry:
		data, err := readEntry(h, r)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &u.manifest); err != nil || u.manifest.Format < 1 {
			return errors.New("not a broker backup: bad manifest")
		}
		if u.manifest.Format > BackupFormat {
			return fmt.Errorf("backup format %d is newer than this broker reads (%d); restore it with a newer broker",
				u.manifest.Format, BackupFormat)
		}
		return nil
	case configEntry:
		var err error
		u.config, err = readEntry(h, r)
		return err
	case publicEntry:
		var err error
		u.public, err = readEntry(h, r)
		return err
	case privateEntry:
		var err error
		u.private, err = readEntry(h, r)
		return err
	case logEntry:
		f, err := os.CreateTemp(u.state, ".log.jsonl.restore*")
		if err != nil {
			return err
		}
		u.log = f.Name()
		_, err = io.Copy(f, r)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}

	rest, inSecrets := strings.CutPrefix(h.Name, "data/secrets/")
	tier, file, _ := strings.Cut(rest, "/")
	ext := path.Ext(file)
	if !inSecrets || (tier != string(Public) && tier != string(Private)) ||
		(ext != ".age" && ext != ".json") || ValidateName(strings.TrimSuffix(file, ext)) != nil {
		return fmt.Errorf("unexpected entry %q in backup", h.Name)
	}
	data, err := readEntry(h, r)
	if err != nil {
		return err
	}
	return paths.WriteFile(filepath.Join(u.stage.TierDir(tier), file), data)
}

func readEntry(h *tar.Header, r io.Reader) ([]byte, error) {
	if h.Size > maxEntry {
		return nil, fmt.Errorf("backup entry %s is too large (%d bytes)", h.Name, h.Size)
	}
	var b bytes.Buffer
	if _, err := io.Copy(&b, r); err != nil {
		return nil, fmt.Errorf("read backup %s: %w", h.Name, err)
	}
	return b.Bytes(), nil
}

// placeLog moves the backup's log into place: as the log when there is no
// local one (or it is empty), beside it otherwise, named for the backup's
// creation time. It returns where it went.
func (u *unpacked) placeLog(local string) (string, error) {
	dst := local
	if fi, err := os.Stat(local); err == nil && fi.Size() > 0 {
		dst = filepath.Join(filepath.Dir(local), "log.restored-"+u.manifest.Created.UTC().Format("20060102T150405Z")+".jsonl")
	}
	if err := os.Chmod(u.log, paths.FileMode); err != nil {
		return "", err
	}
	if err := os.Rename(u.log, dst); err != nil {
		return "", err
	}
	u.log = ""
	return dst, nil
}

// cleanup wipes the private tier identity and removes a staged log that
// was never placed.
func (u *unpacked) cleanup() {
	clear(u.private)
	if u.log != "" {
		os.Remove(u.log)
	}
}
