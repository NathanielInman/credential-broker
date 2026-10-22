package vault

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"filippo.io/age/plugin"

	"github.com/NathanielInman/credential-broker/internal/config"
)

// maxIdentityFile bounds how much of an --identity file is read.
const maxIdentityFile = 1 << 20

// readIdentityFile parses an age identity file into unlock entries, one per
// identity line. Plugin lines (AGE-PLUGIN-...) become stubs; plain
// AGE-SECRET-KEY lines record their recipient. Errors never quote a line,
// since it may be a secret key.
func readIdentityFile(path string, ui *plugin.ClientUI) ([]config.Unlock, []age.Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var unlocks []config.Unlock
	var ids []age.Identity
	sc := bufio.NewScanner(io.LimitReader(f, maxIdentityFile))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "AGE-PLUGIN-") {
			id, err := plugin.NewIdentity(line, ui)
			if err != nil {
				return nil, nil, fmt.Errorf("%s line %d: %w", path, n, err)
			}
			unlocks = append(unlocks, config.Unlock{Path: path, Identity: line})
			ids = append(ids, id)
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
		unlocks = append(unlocks, config.Unlock{Path: path, Recipient: recipient})
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

// unlockRecipients returns the recipients to wrap the private identity to.
// Plugin stubs are used as recipients directly (the plugin derives the
// public key without a touch); plain identities use their stored recipient.
func unlockRecipients(unlocks []config.Unlock, ui *plugin.ClientUI) ([]age.Recipient, error) {
	var rs []age.Recipient
	for _, u := range unlocks {
		if !u.Plain() {
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

// unlockIdentities returns the identities that can unwrap private.key.age.
// Plain identities come first so a plain key never triggers a plugin prompt.
// Unreadable plain identity files are skipped and returned as skipped, so a
// missing test key does not block a working YubiKey.
func unlockIdentities(unlocks []config.Unlock, ui *plugin.ClientUI) (ids []age.Identity, skipped error) {
	var plugins []age.Identity
	var errs []error
	read := map[string]bool{}
	for _, u := range unlocks {
		if !u.Plain() {
			id, err := plugin.NewIdentity(u.Identity, ui)
			if err != nil {
				errs = append(errs, fmt.Errorf("unlock identity from %s: %w", u.Path, err))
				continue
			}
			plugins = append(plugins, id)
			continue
		}
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
			if fu.Plain() {
				ids = append(ids, fileIDs[i])
			}
		}
	}
	return append(ids, plugins...), errors.Join(errs...)
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
