package brokercrypto

import (
	"crypto/md5"
	"encoding/hex"
)

// MD5 returns the hex-encoded MD5 hash of the input string.
func MD5(input string) string {
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}
