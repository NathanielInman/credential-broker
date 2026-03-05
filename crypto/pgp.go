package brokercrypto

import (
	"bytes"

	openpgpArmor "github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/gopenpgp/v3/constants"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/ProtonMail/gopenpgp/v3/profile"
)

// GenerateKey generates an RSA 4096-bit PGP keypair with the given name, email,
// and passphrase. Returns the armored private key, armored public key,
// and armored revocation certificate.
func GenerateKey(name, email, passphrase string) (privateKey, publicKey, revocationCert string, err error) {
	pgp := crypto.PGPWithProfile(profile.RFC4880())

	keyGenHandle := pgp.KeyGeneration().
		AddUserId(name, email).
		New()

	key, err := keyGenHandle.GenerateKeyWithSecurity(constants.HighSecurity)
	if err != nil {
		return "", "", "", err
	}

	// Export public key and locked private key before mutating the entity
	publicKey, err = key.GetArmoredPublicKey()
	if err != nil {
		return "", "", "", err
	}

	lockedKey, err := pgp.LockKey(key, []byte(passphrase))
	if err != nil {
		return "", "", "", err
	}

	privateKey, err = lockedKey.Armor()
	if err != nil {
		return "", "", "", err
	}

	// Generate revocation certificate by revoking the entity in place
	// and serializing the revoked public key. This mutates the entity,
	// so it must happen after the private/public key exports above.
	entity := key.GetEntity()
	err = entity.Revoke(packet.NoReason, "Key revocation", nil)
	if err != nil {
		return "", "", "", err
	}

	var revBuf bytes.Buffer
	armorWriter, err := openpgpArmor.Encode(&revBuf, "PGP PUBLIC KEY BLOCK", nil)
	if err != nil {
		return "", "", "", err
	}
	if err = entity.Serialize(armorWriter); err != nil {
		return "", "", "", err
	}
	if err = armorWriter.Close(); err != nil {
		return "", "", "", err
	}
	revocationCert = revBuf.String()

	return privateKey, publicKey, revocationCert, nil
}

// PGPEncrypt encrypts text with the given armored public key.
// Returns the armored PGP message.
func Encrypt(text, armoredPublicKey string) (string, error) {
	pgp := crypto.PGPWithProfile(profile.RFC4880())

	publicKey, err := crypto.NewKeyFromArmored(armoredPublicKey)
	if err != nil {
		return "", err
	}

	encHandle, err := pgp.Encryption().Recipient(publicKey).New()
	if err != nil {
		return "", err
	}

	pgpMessage, err := encHandle.Encrypt([]byte(text))
	if err != nil {
		return "", err
	}

	armored, err := pgpMessage.Armor()
	if err != nil {
		return "", err
	}

	return armored, nil
}

// PGPDecrypt decrypts an armored PGP message using the given armored private key
// and passphrase. Returns the decrypted plaintext.
func Decrypt(armoredMessage, armoredPrivateKey, passphrase string) (string, error) {
	pgp := crypto.PGPWithProfile(profile.RFC4880())

	privateKey, err := crypto.NewPrivateKeyFromArmored(armoredPrivateKey, []byte(passphrase))
	if err != nil {
		return "", err
	}
	defer privateKey.ClearPrivateParams()

	decHandle, err := pgp.Decryption().DecryptionKey(privateKey).New()
	if err != nil {
		return "", err
	}

	result, err := decHandle.Decrypt([]byte(armoredMessage), crypto.Armor)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// Sign creates a cleartext signed message using the given armored private key
// and passphrase. Returns the armored clearsigned message.
func Sign(text, armoredPrivateKey, passphrase string) (string, error) {
	pgp := crypto.PGPWithProfile(profile.RFC4880())

	privateKey, err := crypto.NewPrivateKeyFromArmored(armoredPrivateKey, []byte(passphrase))
	if err != nil {
		return "", err
	}
	defer privateKey.ClearPrivateParams()

	signHandle, err := pgp.Sign().SigningKey(privateKey).New()
	if err != nil {
		return "", err
	}

	signed, err := signHandle.SignCleartext([]byte(text))
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

// Verify verifies a clearsigned message using the given armored public key.
// Returns the signed data if verification succeeds, or an error if it fails.
func Verify(cleartextMessage, armoredPublicKey string) (string, error) {
	pgp := crypto.PGPWithProfile(profile.RFC4880())

	publicKey, err := crypto.NewKeyFromArmored(armoredPublicKey)
	if err != nil {
		return "", err
	}

	verifyHandle, err := pgp.Verify().VerificationKey(publicKey).New()
	if err != nil {
		return "", err
	}

	result, err := verifyHandle.VerifyCleartext([]byte(cleartextMessage))
	if err != nil {
		return "", err
	}

	if sigErr := result.SignatureError(); sigErr != nil {
		return "", sigErr
	}

	return string(result.Cleartext()), nil
}
