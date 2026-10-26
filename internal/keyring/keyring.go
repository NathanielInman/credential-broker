// Package keyring holds granted secret values where exec reads them back
// without a touch: on Linux the kernel user keyring with the grant agent
// beside it (store_linux.go), elsewhere the grant agent alone.
//
// In the kernel keyring each grant is two keys in the user keyring, both
// with the same kernel timeout: big_key "broker:secret:<NAME>" holds the
// value, and user key "broker:expiry:<NAME>" holds the expiry as Unix
// seconds, because the kernel does not report a key's timeout through
// keyctl. The kernel drops
// both at expiry; there is no daemon.
//
// The user keyring is per uid (and per user namespace), not per login
// session, so a grant made in the human's terminal is visible to agents
// running in other sessions. Keys also carry user view/read/search
// permission, so a process that does not possess them can still read them.
package keyring

import (
	"errors"
	"time"
)

var (
	// ErrNotGranted means name has no grant, or it was revoked, or it
	// expired long enough ago that the kernel collected it.
	ErrNotGranted = errors.New("not granted")
	// ErrExpired means name's grant timed out recently. The kernel keeps
	// expired keys until garbage collection (kernel.keys.gc_delay, 5
	// minutes by default); after that they read as ErrNotGranted.
	ErrExpired = errors.New("grant expired")
)

// Grant is a live grant.
type Grant struct {
	Name string
	// Expires is when the kernel drops the grant. It is zero if the
	// companion expiry key is missing.
	Expires time.Time
}
