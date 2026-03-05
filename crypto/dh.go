package brokercrypto

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
)

// GenerateKeyPair generates a Diffie-Hellman keypair with a 512-bit prime.
// Returns the prime, public key, and private key as base64-encoded strings.
func GenerateKeyPair() (prime, publicKey, privateKey string, err error) {
	// Generate a 512-bit safe prime p
	p, err := rand.Prime(rand.Reader, 512)
	if err != nil {
		return "", "", "", err
	}

	// Use generator g = 2 (standard DH generator)
	g := big.NewInt(2)

	// Generate a random private key in [2, p-2]
	// pMinus2 = p - 2
	pMinus2 := new(big.Int).Sub(p, big.NewInt(2))
	privKey, err := rand.Int(rand.Reader, pMinus2)
	if err != nil {
		return "", "", "", err
	}
	// Shift to [2, p-1) to avoid trivial keys
	privKey.Add(privKey, big.NewInt(2))

	// Compute public key: g^privKey mod p
	pubKey := new(big.Int).Exp(g, privKey, p)

	prime = base64.StdEncoding.EncodeToString(p.Bytes())
	publicKey = base64.StdEncoding.EncodeToString(pubKey.Bytes())
	privateKey = base64.StdEncoding.EncodeToString(privKey.Bytes())

	return prime, publicKey, privateKey, nil
}

// ComputeSecret computes the Diffie-Hellman shared secret from base64-encoded
// prime, the other party's public key, and the local private key.
// Returns the shared secret as a base64-encoded string.
func ComputeSecret(prime, otherPublicKey, privateKey string) (string, error) {
	primeBytes, err := base64.StdEncoding.DecodeString(prime)
	if err != nil {
		return "", err
	}

	otherPubBytes, err := base64.StdEncoding.DecodeString(otherPublicKey)
	if err != nil {
		return "", err
	}

	privBytes, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}

	p := new(big.Int).SetBytes(primeBytes)
	otherPub := new(big.Int).SetBytes(otherPubBytes)
	priv := new(big.Int).SetBytes(privBytes)

	// shared secret = otherPub^priv mod p
	secret := new(big.Int).Exp(otherPub, priv, p)

	return base64.StdEncoding.EncodeToString(secret.Bytes()), nil
}

// GenerateKeyPairFromPrime generates a DH keypair using an existing prime.
func GenerateKeyPairFromPrime(primeB64 string) (publicKey, privateKey string, err error) {
	primeBytes, err := base64.StdEncoding.DecodeString(primeB64)
	if err != nil {
		return "", "", err
	}

	p := new(big.Int).SetBytes(primeBytes)
	g := big.NewInt(2)

	pMinus2 := new(big.Int).Sub(p, big.NewInt(2))
	privKey, err := rand.Int(rand.Reader, pMinus2)
	if err != nil {
		return "", "", err
	}
	privKey.Add(privKey, big.NewInt(2))

	pubKey := new(big.Int).Exp(g, privKey, p)

	publicKey = base64.StdEncoding.EncodeToString(pubKey.Bytes())
	privateKey = base64.StdEncoding.EncodeToString(privKey.Bytes())

	return publicKey, privateKey, nil
}

// DHFromPrime takes a base64-encoded prime and client public key,
// generates server keypair, and computes shared secret.
// Returns server public key and shared secret as base64 strings.
func DHFromPrime(primeB64, clientPubB64 string) (serverPub, secret string, err error) {
	serverPubKey, serverPrivKey, err := GenerateKeyPairFromPrime(primeB64)
	if err != nil {
		return "", "", err
	}

	sharedSecret, err := ComputeSecret(primeB64, clientPubB64, serverPrivKey)
	if err != nil {
		return "", "", err
	}

	return serverPubKey, sharedSecret, nil
}

// RandomBase64 generates n random bytes and returns them as base64.
func RandomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
