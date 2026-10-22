package vault

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"filippo.io/age/plugin"

	"github.com/NathanielInman/credential-broker/internal/config"
)

// maxIdentityFile bounds how much of an --identity file is read.
const maxIdentityFile = 1 << 20

// readIdentityFile parses an age identity file into unlock entries, one per
// identity line. Plugin lines (AGE-PLUGIN-...) become stubs; plain
// AGE-SECRET-KEY lines record their recipient. A file encrypted with a
// passphrase (age -p) is decrypted first, asking through ui, and its
// entries are marked Passphrase. Errors never quote a line, since it may be
// a secret key.
func readIdentityFile(path string, ui *plugin.ClientUI) ([]config.Unlock, []age.Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxIdentityFile))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	passphrase := isEncrypted(data)
	if passphrase {
		if data, err = decryptIdentityFile(path, data, ui); err != nil {
			return nil, nil, err
		}
		defer clear(data)
	}

	var unlocks []config.Unlock
	var ids []age.Identity
	// noted is the recipient a plugin wrote in a comment above its stub.
	var noted string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if m := recipientComment.FindStringSubmatch(line); m != nil {
			noted = m[1]
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "AGE-PLUGIN-") {
			if passphrase {
				return nil, nil, fmt.Errorf("%s line %d: a plugin identity inside a passphrase file is not supported", path, n)
			}
			id, err := plugin.NewIdentity(line, ui)
			if err != nil {
				return nil, nil, fmt.Errorf("%s line %d: %w", path, n, err)
			}
			unlocks = append(unlocks, config.Unlock{Path: path, Identity: line, Recipient: noted})
			ids = append(ids, id)
			noted = ""
			continue
		}
		parsed, err := age.ParseIdentities(strings.NewReader(line))
		if err != nil {
			return nil, nil, fmt.Errorf("%s line %d: not an age identity", path, n)
		}
		var recipient string
		switch id := parsed[0].(type) {
		case *age.X25519Identity:
			recipient = id.Recipient().String()
		case *age.HybridIdentity:
			recipient = id.Recipient().String()
		default:
			return nil, nil, fmt.Errorf("%s line %d: unsupported identity type %T", path, n, id)
		}
		unlocks = append(unlocks, config.Unlock{Path: path, Recipient: recipient, Passphrase: passphrase})
		ids = append(ids, parsed[0])
	}
	if err := sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(unlocks) == 0 {
		return nil, nil, fmt.Errorf("%s: no age identities found", path)
	}
	return unlocks, ids, nil
}

// recipientComment matches the recipient a plugin writes beside its stub:
// "# public key: age1se1..." (age-plugin-se) or "#    Recipient: age1yubikey1..."
// (age-plugin-yubikey).
var recipientComment = regexp.MustCompile(`^#\s*(?i:public key|recipient):\s*(age1[0-9a-z]+)\s*$`)

// isEncrypted reports whether data is an age file (binary or armored)
// rather than an identity file.
func isEncrypted(data []byte) bool {
	data = bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(data, []byte("age-encryption.org/")) || bytes.HasPrefix(data, []byte(armor.Header))
}

// decryptIdentityFile asks for path's passphrase through ui and returns
// the identity file inside. The caller clears the result.
func decryptIdentityFile(path string, data []byte, ui *plugin.ClientUI) ([]byte, error) {
	pass, err := ui.RequestValue("broker", fmt.Sprintf("passphrase for %s:", path), true)
	if err != nil {
		return nil, fmt.Errorf("%s: read passphrase: %w", path, err)
	}
	if pass == "" {
		return nil, fmt.Errorf("%s: the passphrase is empty", path)
	}
	id, err := age.NewScryptIdentity(pass)
	if err != nil {
		return nil, err
	}
	var r io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte(armor.Header)) {
		r = armor.NewReader(r)
	}
	dr, err := age.Decrypt(r, id)
	if err != nil {
		// Not wrapped: a wrong passphrase must not read as "this identity
		// does not match", or decrypting would move on and hide it.
		return nil, fmt.Errorf("%s: wrong passphrase, or not encrypted with one (age -p)", path)
	}
	out, err := io.ReadAll(io.LimitReader(dr, maxIdentityFile))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

// passphraseIdentity is a passphrase-encrypted identity file. It asks for
// the passphrase only when a stanza it could open is reached, so a vault
// that also unlocks with hardware never prompts for it needlessly.
type passphraseIdentity struct {
	path string
	ui   *plugin.ClientUI
}

func (p *passphraseIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	native := false
	for _, s := range stanzas {
		native = native || s.Type == "X25519" || s.Type == "mlkem768x25519"
	}
	if !native {
		return nil, age.ErrIncorrectIdentity
	}
	_, ids, err := readIdentityFile(p.path, p.ui)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		key, err := id.Unwrap(stanzas)
		if errors.Is(err, age.ErrIncorrectIdentity) {
			continue
		}
		return key, err
	}
	return nil, fmt.Errorf("%s: %w", p.path, age.ErrIncorrectIdentity)
}

// unlockRecipients returns the recipients to wrap the private identity to.
// A plugin stub uses the recipient its plugin wrote beside it, or else the
// stub itself, from which the plugin derives the public key (no touch
// either way); plain and passphrase identities use their stored recipient.
func unlockRecipients(unlocks []config.Unlock, ui *plugin.ClientUI) ([]age.Recipient, error) {
	var rs []age.Recipient
	for _, u := range unlocks {
		if u.Zone() == config.ZoneHardware && u.Recipient != "" {
			r, err := plugin.NewRecipient(u.Recipient, ui)
			if err != nil {
				return nil, fmt.Errorf("unlock recipient for %s: %w", u.Path, err)
			}
			rs = append(rs, r)
			continue
		}
		if u.Zone() == config.ZoneHardware {
			id, err := plugin.NewIdentity(u.Identity, ui)
			if err != nil {
				return nil, fmt.Errorf("unlock identity from %s: %w", u.Path, err)
			}
			rs = append(rs, id.Recipient())
			continue
		}
		parsed, err := age.ParseRecipients(strings.NewReader(u.Recipient))
		if err != nil {
			return nil, fmt.Errorf("unlock recipient for %s: %w", u.Path, err)
		}
		rs = append(rs, parsed...)
	}
	if len(rs) == 0 {
		return nil, errors.New("no unlock identities")
	}
	return rs, nil
}

// unlockIdentities returns the identities that can unwrap private.key.age,
// in two rounds: plain and plugin identities first (age tries plain ones
// before any plugin, so a plain key never triggers a prompt), then
// passphrase files, which are only worth asking for if the first round
// fails (no hardware plugged in). Unreadable files are skipped and returned
// as skipped, so a missing test key does not block a working YubiKey.
func unlockIdentities(unlocks []config.Unlock, ui *plugin.ClientUI) (first, then []age.Identity, skipped error) {
	var plugins []age.Identity
	var errs []error
	read := map[string]bool{}
	for _, u := range unlocks {
		switch u.Zone() {
		case config.ZoneHardware:
			id, err := plugin.NewIdentity(u.Identity, ui)
			if err != nil {
				errs = append(errs, fmt.Errorf("unlock identity from %s: %w", u.Path, err))
				continue
			}
			plugins = append(plugins, id)
		case config.ZonePassphrase:
			if read[u.Path] {
				continue
			}
			read[u.Path] = true
			if _, err := os.Stat(u.Path); err != nil {
				errs = append(errs, err)
				continue
			}
			then = append(then, &passphraseIdentity{path: u.Path, ui: ui})
		default:
			if read[u.Path] {
				continue
			}
			read[u.Path] = true
			fileUnlocks, fileIDs, err := readIdentityFile(u.Path, ui)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for i, fu := range fileUnlocks {
				if fu.Zone() == config.ZonePlain {
					first = append(first, fileIDs[i])
				}
			}
		}
	}
	return append(first, plugins...), then, errors.Join(errs...)
}

// wrapPrivate encrypts the private tier identity file to the unlock
// identities.
func wrapPrivate(identityFile []byte, unlocks []config.Unlock, ui *plugin.ClientUI) ([]byte, error) {
	rs, err := unlockRecipients(unlocks, ui)
	if err != nil {
		return nil, err
	}
	return encrypt(identityFile, rs...)
}

// identityFile renders a tier identity in age-keygen's file format, so the
// stock age CLI can use it directly.
func identityFile(id *age.X25519Identity, tier Tier) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# broker %s tier identity\n", tier)
	fmt.Fprintf(&b, "# public key: %s\n", id.Recipient())
	fmt.Fprintf(&b, "%s\n", id)
	return b.Bytes()
}

// parseTierIdentity parses an identityFile and checks it matches recipient.
func parseTierIdentity(data []byte, recipient string) (*age.X25519Identity, error) {
	ids, err := age.ParseIdentities(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	id, ok := ids[0].(*age.X25519Identity)
	if len(ids) != 1 || !ok {
		return nil, errors.New("want exactly one X25519 identity")
	}
	if id.Recipient().String() != recipient {
		return nil, fmt.Errorf("identity does not match recipient %s in config", recipient)
	}
	return id, nil
}

func encrypt(plaintext []byte, rs ...age.Recipient) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, rs...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decryptFile(path string, ids ...age.Identity) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := age.Decrypt(f, ids...)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}
