package keyring

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	secretPrefix = "broker:secret:"
	expiryPrefix = "broker:expiry:"
	// big_key holds up to 1 MiB (vault.MaxValue fits) and moves large
	// payloads out of the key quota into encrypted shmem; a user key caps
	// at 32 KiB and counts every byte against kernel.keys.maxbytes.
	secretType = "big_key"
	expiryType = "user"
)

// Key permission bits (linux/keyctl.h; x/sys/unix does not export them).
const (
	posAll     = 0x3f000000
	usrView    = 0x00010000
	usrRead    = 0x00020000
	usrSearch  = 0x00080000
	permission = posAll | usrView | usrRead | usrSearch
)

// Keyring is the calling uid's kernel user keyring.
type Keyring struct {
	ring int
}

// User returns the user keyring, creating it if the kernel has not yet.
// Every call addresses it as KEY_SPEC_USER_KEYRING, which the caller
// always possesses whatever its session keyring is.
func User() (*Keyring, error) {
	if _, err := unix.KeyctlGetKeyringID(unix.KEY_SPEC_USER_KEYRING, true); err != nil {
		return nil, fmt.Errorf("kernel user keyring: %w", err)
	}
	return &Keyring{ring: unix.KEY_SPEC_USER_KEYRING}, nil
}

// Add grants name for ttl (rounded up to whole seconds) and returns when
// the grant expires. An existing grant of name is replaced atomically.
func (k *Keyring) Add(name string, value []byte, ttl time.Duration) (time.Time, error) {
	secs := int64((ttl + time.Second - 1) / time.Second)
	if secs < 1 {
		return time.Time{}, fmt.Errorf("grant duration %s is not positive", ttl)
	}
	// The thread keyring stages keys until they have their timeout, so a
	// crash never leaves a timeless secret in the user keyring.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// The kernel sets expiry to its current second plus the timeout.
	expires := time.Unix(time.Now().Unix()+secs, 0)
	stamp := []byte(strconv.FormatInt(expires.Unix(), 10))
	if err := k.add(expiryType, expiryPrefix+name, stamp, secs); err != nil {
		return time.Time{}, fmt.Errorf("grant %s: %w", name, err)
	}
	if err := k.add(secretType, secretPrefix+name, value, secs); err != nil {
		return time.Time{}, fmt.Errorf("grant %s: %w", name, err)
	}
	return expires, nil
}

// add creates a key in the thread keyring, sets its timeout and
// permissions, then links it into k. Linking displaces any key with the
// same type and description, so readers never see a gap.
func (k *Keyring) add(typ, desc string, payload []byte, secs int64) error {
	id, err := unix.AddKey(typ, desc, payload, unix.KEY_SPEC_THREAD_KEYRING)
	if err != nil {
		return keyErr(err)
	}
	defer unix.KeyctlInt(unix.KEYCTL_UNLINK, id, unix.KEY_SPEC_THREAD_KEYRING, 0, 0)
	if _, err := unix.KeyctlInt(unix.KEYCTL_SET_TIMEOUT, id, int(secs), 0, 0); err != nil {
		return keyErr(err)
	}
	if err := unix.KeyctlSetperm(id, permission); err != nil {
		return keyErr(err)
	}
	if _, err := unix.KeyctlInt(unix.KEYCTL_LINK, id, k.ring, 0, 0); err != nil {
		return keyErr(err)
	}
	return nil
}

func keyErr(err error) error {
	if errors.Is(err, unix.EDQUOT) {
		return fmt.Errorf("%w: kernel key quota exceeded (sysctl kernel.keys.maxkeys / kernel.keys.maxbytes)", err)
	}
	return err
}

// Read returns name's granted value, ErrNotGranted, or ErrExpired.
func (k *Keyring) Read(name string) ([]byte, error) {
	id, err := k.search(secretType, secretPrefix+name)
	if err != nil {
		return nil, err
	}
	return readKey(id)
}

// Lookup returns name's live grant, ErrNotGranted, or ErrExpired.
func (k *Keyring) Lookup(name string) (Grant, error) {
	if _, err := k.search(secretType, secretPrefix+name); err != nil {
		return Grant{}, err
	}
	g := Grant{Name: name}
	if id, err := k.search(expiryType, expiryPrefix+name); err == nil {
		if stamp, err := readKey(id); err == nil {
			if sec, err := strconv.ParseInt(string(stamp), 10, 64); err == nil {
				g.Expires = time.Unix(sec, 0)
			}
		}
	}
	return g, nil
}

func (k *Keyring) search(typ, desc string) (int, error) {
	id, err := unix.KeyctlSearch(k.ring, typ, desc, 0)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, unix.ENOKEY), errors.Is(err, unix.EKEYREVOKED):
		return 0, ErrNotGranted
	case errors.Is(err, unix.EKEYEXPIRED):
		return 0, ErrExpired
	}
	return 0, err
}

// readKey reads a key's payload, growing the buffer if it changed size
// between calls.
func readKey(id int) ([]byte, error) {
	var buf []byte
	for {
		n, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
		switch {
		case errors.Is(err, unix.ENOKEY), errors.Is(err, unix.EKEYREVOKED):
			return nil, ErrNotGranted
		case errors.Is(err, unix.EKEYEXPIRED):
			return nil, ErrExpired
		case err != nil:
			return nil, err
		}
		if n <= len(buf) {
			return buf[:n], nil
		}
		clear(buf)
		buf = make([]byte, n)
	}
}

// List returns every live grant in k, in keyring order.
func (k *Keyring) List() ([]Grant, error) {
	links, err := k.links()
	if err != nil {
		return nil, err
	}
	var out []Grant
	for _, l := range links {
		name, ok := strings.CutPrefix(l.desc, secretPrefix)
		if !ok || l.typ != secretType {
			continue
		}
		g, err := k.Lookup(name)
		if errors.Is(err, ErrNotGranted) || errors.Is(err, ErrExpired) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// Revoke unlinks name's keys and reports whether a live grant was dropped.
// Expired keys cannot be described, so they stay until the kernel collects
// them; a later Add still displaces them.
func (k *Keyring) Revoke(name string) (bool, error) {
	_, err := k.Lookup(name)
	live := err == nil
	if err != nil && !errors.Is(err, ErrNotGranted) && !errors.Is(err, ErrExpired) {
		return false, err
	}
	links, err := k.links()
	if err != nil {
		return false, err
	}
	for _, l := range links {
		if (l.typ == secretType && l.desc == secretPrefix+name) ||
			(l.typ == expiryType && l.desc == expiryPrefix+name) {
			_, err := unix.KeyctlInt(unix.KEYCTL_UNLINK, l.id, k.ring, 0, 0)
			if err != nil && !errors.Is(err, unix.ENOENT) {
				return live, fmt.Errorf("revoke %s: %w", name, err)
			}
		}
	}
	return live, nil
}

type link struct {
	id        int
	typ, desc string
}

// links lists the keys linked from k, skipping any it cannot describe.
func (k *Keyring) links() ([]link, error) {
	ids, err := readKey(k.ring)
	if err != nil {
		return nil, fmt.Errorf("read user keyring: %w", err)
	}
	var out []link
	for i := 0; i+4 <= len(ids); i += 4 {
		id := int(int32(binary.NativeEndian.Uint32(ids[i:])))
		// type;uid;gid;perm;description
		d, err := unix.KeyctlString(unix.KEYCTL_DESCRIBE, id)
		if err != nil {
			continue
		}
		f := strings.SplitN(d, ";", 5)
		if len(f) == 5 {
			out = append(out, link{id: id, typ: f[0], desc: f[4]})
		}
	}
	return out, nil
}
