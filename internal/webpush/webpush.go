// Package webpush sends Web Push notifications: the message is encrypted
// to the browser's subscription keys (RFC 8291, aes128gcm from RFC 8188),
// so the push service relaying it (Google's, for Chrome) cannot read it,
// and the request is signed with the sender's VAPID key (RFC 8292).
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ErrGone means the push service no longer knows the subscription: the
// browser dropped it (notifications turned off, site data cleared).
var ErrGone = errors.New("the push subscription is gone")

// Subscription is what the browser's PushManager.subscribe returns:
// where to send, and the keys to encrypt to, base64url as browsers give
// them.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Key is a VAPID key, which identifies the sender to the push service.
// The browser subscribed with its public half, and the push service
// refuses messages signed by any other.
type Key struct {
	priv *ecdsa.PrivateKey
}

// GenerateKey makes a new VAPID key.
func GenerateKey() (*Key, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Key{priv}, nil
}

// ParseKey reads a key MarshalText wrote.
func ParseKey(text []byte) (*Key, error) {
	raw, err := b64.DecodeString(string(bytes.TrimSpace(text)))
	if err != nil {
		return nil, fmt.Errorf("VAPID key: %w", err)
	}
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return nil, fmt.Errorf("VAPID key: %w", err)
	}
	return &Key{priv}, nil
}

// MarshalText is the private key, base64url.
func (k *Key) MarshalText() ([]byte, error) {
	raw, err := k.priv.Bytes()
	if err != nil {
		return nil, err
	}
	return []byte(b64.EncodeToString(raw)), nil
}

// Public is the public key in the form PushManager.subscribe takes as
// applicationServerKey: an uncompressed point, base64url.
func (k *Key) Public() string {
	pub, _ := k.priv.PublicKey.Bytes()
	return b64.EncodeToString(pub)
}

var b64 = base64.RawURLEncoding

// decode reads base64url with or without padding, as browsers vary.
func decode(s string) ([]byte, error) {
	if b, err := b64.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// recordSize is the aes128gcm record size the header announces. A push
// message is one record, well under it.
const recordSize = 4096

// Encrypt encrypts plaintext for sub as an aes128gcm body.
func Encrypt(sub Subscription, plaintext []byte) ([]byte, error) {
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return encrypt(sub, plaintext, as, salt)
}

func encrypt(sub Subscription, plaintext []byte, as *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	uaRaw, err := decode(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("subscription p256dh: %w", err)
	}
	// NewPublicKey checks the point is on the curve, which RFC 8291
	// requires before using it.
	ua, err := ecdh.P256().NewPublicKey(uaRaw)
	if err != nil {
		return nil, fmt.Errorf("subscription p256dh: %w", err)
	}
	auth, err := decode(sub.Auth)
	if err != nil || len(auth) != 16 {
		return nil, errors.New("subscription auth: want 16 bytes, base64url")
	}
	secret, err := as.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPub := as.PublicKey().Bytes()

	// The shared secret and the auth secret make the input key; the
	// record's salt makes the content key and nonce from that.
	keyInfo := append(append([]byte("WebPush: info\x00"), uaRaw...), asPub...)
	prkKey, err := hkdf.Extract(sha256.New, secret, auth)
	if err != nil {
		return nil, err
	}
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Header: salt, record size, and the sender's key as the key ID.
	body := append([]byte{}, salt...)
	body = binary.BigEndian.AppendUint32(body, recordSize)
	body = append(body, byte(len(asPub)))
	body = append(body, asPub...)
	// One record, so the last: plaintext and the 0x02 delimiter.
	return gcm.Seal(body, nonce, append(append([]byte{}, plaintext...), 2), nil), nil
}

// Message is one push to send.
type Message struct {
	// Payload is what the service worker's push event gets.
	Payload []byte
	// TTL is how long the push service keeps it for an offline phone.
	TTL time.Duration
	// Topic lets a newer message replace one still waiting.
	Topic string
	// Urgent asks the push service to deliver now, past battery saving.
	Urgent bool
}

// Send encrypts m to sub and posts it to sub's push service, signed with
// key. Subject is a contact URL (mailto: or https:) for the push service.
func Send(ctx context.Context, client *http.Client, sub Subscription, m Message, key *Key, subject string) error {
	endpoint, err := url.Parse(sub.Endpoint)
	if err != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.Host == "" {
		return fmt.Errorf("push endpoint %q is not a URL", sub.Endpoint)
	}
	body, err := Encrypt(sub, m.Payload)
	if err != nil {
		return err
	}
	jwt, err := key.sign(endpoint.Scheme+"://"+endpoint.Host, subject, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(int(m.TTL/time.Second)))
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+key.Public())
	if m.Topic != "" {
		req.Header.Set("Topic", m.Topic)
	}
	if m.Urgent {
		req.Header.Set("Urgency", "high")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode/100 != 2:
		return fmt.Errorf("push service: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	return nil
}

// sign makes the VAPID JWT for audience, valid for 12 hours (the most a
// push service accepts is 24).
func (k *Key) sign(audience, subject string, now time.Time) (string, error) {
	header := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, err := json.Marshal(map[string]any{
		"aud": audience,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": subject,
	})
	if err != nil {
		return "", err
	}
	signing := header + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, digest[:])
	if err != nil {
		return "", err
	}
	// ES256 wants r and s as two 32-byte big-endian halves, not DER.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signing + "." + b64.EncodeToString(sig), nil
}
