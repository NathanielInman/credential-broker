package brokercrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
)

// AESEncrypt encrypts content using AES-256-CBC with the first 32 bytes of secret
// as the key. Returns URL-encoded "hex(iv):base64(ciphertext)".
// Panics on crypto errors (they indicate programming bugs, not runtime conditions).
func AESEncrypt(content, secret string) string {
	key := []byte(secret[:32])

	iv := make([]byte, 16)
	rand.Read(iv)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	plaintext := pkcs7Pad([]byte(content), aes.BlockSize)
	ciphertext := make([]byte, len(plaintext))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	ivHex := hex.EncodeToString(iv)
	ctBase64 := base64.StdEncoding.EncodeToString(ciphertext)

	return url.QueryEscape(ivHex + ":" + ctBase64)
}

// AESDecrypt decrypts URL-encoded "hex(iv):base64(ciphertext)" using AES-256-CBC
// with secret[:32] as the key. Returns plaintext string.
func AESDecrypt(content, secret string) (string, error) {
	key := []byte(secret[:32])

	decoded, err := url.QueryUnescape(content)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid encrypted content format: expected 'iv:ciphertext'")
	}

	iv, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	plaintext, err := pkcs7Unpad(ciphertext, aes.BlockSize)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs7: data is empty")
	}
	if len(data)%blockSize != 0 {
		return nil, errors.New("pkcs7: data is not a multiple of block size")
	}

	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("pkcs7: invalid padding value")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("pkcs7: invalid padding bytes")
		}
	}

	return data[:len(data)-padding], nil
}
