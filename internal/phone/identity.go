package phone

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"strings"

	"filippo.io/age"
)

// identityInfo binds the derivation to its purpose, so the same PRF
// output used elsewhere never yields this key.
const identityInfo = "broker phone unlock v1"

// Identity derives the age X25519 identity a security key's PRF output
// stands for. It is an ordinary age identity: anyone who can get the PRF
// output from the key can unwrap the vault with the stock age CLI.
func Identity(prf []byte) (*age.X25519Identity, error) {
	if len(prf) != 32 {
		return nil, errors.New("a PRF output is 32 bytes")
	}
	scalar, err := hkdf.Key(sha256.New, prf, nil, identityInfo, 32)
	if err != nil {
		return nil, err
	}
	defer clear(scalar)
	// age has no constructor from raw bytes, so this goes through the
	// identity's text form, which ParseX25519Identity checks.
	return age.ParseX25519Identity(strings.ToUpper(bech32("age-secret-key-", scalar)))
}

// bech32 encodes data as BIP 173 Bech32 under the lowercase hrp.
func bech32(hrp string, data []byte) string {
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	// Regroup 8-bit bytes into 5-bit values, zero padding the last.
	var values []byte
	acc, bits := 0, 0
	for _, b := range data {
		acc = acc<<8 | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			values = append(values, byte(acc>>bits)&31)
		}
	}
	if bits > 0 {
		values = append(values, byte(acc<<(5-bits))&31)
	}
	// The checksum covers the hrp's high bits, a zero, its low bits, the
	// values, and six zeros for the checksum itself.
	var check []byte
	for _, c := range []byte(hrp) {
		check = append(check, c>>5)
	}
	check = append(check, 0)
	for _, c := range []byte(hrp) {
		check = append(check, c&31)
	}
	check = append(append(check, values...), 0, 0, 0, 0, 0, 0)
	mod := polymod(check) ^ 1

	var b strings.Builder
	b.WriteString(hrp)
	b.WriteByte('1')
	for _, v := range values {
		b.WriteByte(charset[v])
	}
	for i := range 6 {
		b.WriteByte(charset[mod>>(5*(5-i))&31])
	}
	return b.String()
}

func polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := range 5 {
			if top>>i&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}
