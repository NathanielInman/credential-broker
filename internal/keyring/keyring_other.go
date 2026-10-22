//go:build !linux && !darwin && !windows

package keyring

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("the kernel keyring is linux only")

// Keyring is unavailable off linux; User always fails.
type Keyring struct{}

func User() (*Keyring, error) { return nil, errUnsupported }

func (*Keyring) Add(string, []byte, time.Duration) (time.Time, error) {
	return time.Time{}, errUnsupported
}
func (*Keyring) Read(string) ([]byte, error)  { return nil, errUnsupported }
func (*Keyring) Lookup(string) (Grant, error) { return Grant{}, errUnsupported }
func (*Keyring) List() ([]Grant, error)       { return nil, errUnsupported }
func (*Keyring) Revoke(string) (bool, error)  { return false, errUnsupported }
