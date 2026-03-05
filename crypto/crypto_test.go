package brokercrypto

import (
	"strings"
	"testing"
)

func TestMD5(t *testing.T) {
	// Known MD5 values
	if got := MD5("hello"); got != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("MD5(hello) = %s, want 5d41402abc4b2a76b9719d911017c592", got)
	}
	if got := MD5(""); got != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("MD5('') = %s, want d41d8cd98f00b204e9800998ecf8427e", got)
	}
}

func TestAESRoundTrip(t *testing.T) {
	// Secret must be at least 32 chars
	secret := "01234567890123456789012345678901"

	tests := []string{
		"hello world",
		"",
		"a longer message with special chars: !@#$%^&*()",
		`{"key": "value", "nested": {"a": 1}}`,
	}

	for _, msg := range tests {
		encrypted := AESEncrypt(msg, secret)
		decrypted, err := AESDecrypt(encrypted, secret)
		if err != nil {
			t.Fatalf("AESDecrypt error for %q: %v", msg, err)
		}
		if decrypted != msg {
			t.Errorf("AES roundtrip failed: got %q, want %q", decrypted, msg)
		}
	}
}

func TestAESEncryptFormat(t *testing.T) {
	secret := "01234567890123456789012345678901"
	encrypted := AESEncrypt("test", secret)

	// Should be URL-encoded, so no raw colons
	if strings.Contains(encrypted, ":") {
		t.Error("AESEncrypt output should be URL-encoded (no raw colons)")
	}
}

func TestDHKeyExchange(t *testing.T) {
	// Client generates keypair
	prime, clientPub, clientPriv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}

	// Server generates from same prime
	serverPub, serverPriv, err := GenerateKeyPairFromPrime(prime)
	if err != nil {
		t.Fatalf("GenerateKeyPairFromPrime error: %v", err)
	}

	// Both compute shared secret
	clientSecret, err := ComputeSecret(prime, serverPub, clientPriv)
	if err != nil {
		t.Fatalf("Client ComputeSecret error: %v", err)
	}

	serverSecret, err := ComputeSecret(prime, clientPub, serverPriv)
	if err != nil {
		t.Fatalf("Server ComputeSecret error: %v", err)
	}

	if clientSecret != serverSecret {
		t.Errorf("DH secrets don't match: client=%s, server=%s", clientSecret, serverSecret)
	}
}

func TestDHFromPrime(t *testing.T) {
	prime, clientPub, clientPriv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}

	serverPub, secret, err := DHFromPrime(prime, clientPub)
	if err != nil {
		t.Fatalf("DHFromPrime error: %v", err)
	}

	if serverPub == "" || secret == "" {
		t.Error("DHFromPrime returned empty values")
	}

	clientSecret, err := ComputeSecret(prime, serverPub, clientPriv)
	if err != nil {
		t.Fatalf("ComputeSecret error: %v", err)
	}

	if clientSecret != secret {
		t.Error("DHFromPrime secret doesn't match client computation")
	}
}

func TestDHAndAESIntegration(t *testing.T) {
	// Full DH + AES flow like the actual auth
	prime, clientPub, clientPriv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverPub, serverSecret, err := DHFromPrime(prime, clientPub)
	if err != nil {
		t.Fatal(err)
	}

	clientSecret, err := ComputeSecret(prime, serverPub, clientPriv)
	if err != nil {
		t.Fatal(err)
	}

	// Both sides should be able to encrypt/decrypt with the shared secret
	// Pad secret to 32 chars if needed
	if len(clientSecret) < 32 {
		t.Skipf("Secret too short (%d chars), skipping AES test", len(clientSecret))
	}

	msg := "authenticated"
	encrypted := AESEncrypt(msg, serverSecret)
	decrypted, err := AESDecrypt(encrypted, clientSecret)
	if err != nil {
		t.Fatalf("Cross-party AES decrypt error: %v", err)
	}
	if decrypted != msg {
		t.Errorf("Cross-party AES: got %q, want %q", decrypted, msg)
	}
}

func TestPGPRoundTrip(t *testing.T) {
	priv, pub, revoc, err := GenerateKey("test", "test@test.com", "passphrase")
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}

	if priv == "" || pub == "" || revoc == "" {
		t.Fatal("GenerateKey returned empty values")
	}

	// Encrypt/decrypt
	msg := "secret message"
	encrypted, err := Encrypt(msg, pub)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	decrypted, err := Decrypt(encrypted, priv, "passphrase")
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	if decrypted != msg {
		t.Errorf("PGP roundtrip: got %q, want %q", decrypted, msg)
	}
}

func TestPGPSignVerify(t *testing.T) {
	priv, pub, _, err := GenerateKey("test", "test@test.com", "passphrase")
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}

	msg := `{"key": "value"}`
	signed, err := Sign(msg, priv, "passphrase")
	if err != nil {
		t.Fatalf("Sign error: %v", err)
	}

	verified, err := Verify(signed, pub)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}

	if strings.TrimSpace(verified) != msg {
		t.Errorf("Sign/Verify roundtrip: got %q, want %q", verified, msg)
	}
}

func TestRandomBase64(t *testing.T) {
	result, err := RandomBase64(256)
	if err != nil {
		t.Fatalf("RandomBase64 error: %v", err)
	}
	if len(result) == 0 {
		t.Error("RandomBase64 returned empty string")
	}
}
