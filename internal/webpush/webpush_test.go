package webpush

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := b64.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestEncryptRFC8291 checks Encrypt against the example in RFC 8291
// section 5 and appendix A.
func TestEncryptRFC8291(t *testing.T) {
	sub := Subscription{
		P256dh: "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:   "BTBZMqHH6r4Tts7J_aSIgg",
	}
	as, err := ecdh.P256().NewPrivateKey(mustB64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := encrypt(sub, []byte("When I grow up, I want to be a watermelon"), as, mustB64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := mustB64(t, `DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml
		mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT
		pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN`)
	if b64.EncodeToString(got) != b64.EncodeToString(want) {
		t.Fatalf("encrypt =\n%s\nwant\n%s", b64.EncodeToString(got), b64.EncodeToString(want))
	}
}

func TestKeyRoundTrip(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	text, err := k.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ParseKey(text)
	if err != nil {
		t.Fatal(err)
	}
	if k.Public() != k2.Public() || len(mustB64(t, k.Public())) != 65 {
		t.Fatalf("public %s, after round trip %s", k.Public(), k2.Public())
	}
	if _, err := ParseKey([]byte("not a key")); err == nil {
		t.Fatal("ParseKey accepted junk")
	}
}

func TestSend(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ua, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var got *http.Request
	var body []byte
	status := http.StatusCreated
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	defer srv.Close()
	sub := Subscription{
		Endpoint: srv.URL + "/push/abc",
		P256dh:   b64.EncodeToString(ua.PublicKey().Bytes()),
		Auth:     b64.EncodeToString(make([]byte, 16)),
	}
	m := Message{Payload: []byte(`{"title":"broker"}`), TTL: 2 * time.Minute, Topic: "request", Urgent: true}
	if err := Send(context.Background(), srv.Client(), sub, m, key, "https://broker.example"); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"TTL": "120", "Content-Encoding": "aes128gcm", "Topic": "request", "Urgency": "high"} {
		if got.Header.Get(k) != v {
			t.Errorf("header %s = %q, want %q", k, got.Header.Get(k), v)
		}
	}
	// 16 salt + 4 record size + 1 + 65 key + payload + delimiter + 16 tag.
	if want := 86 + len(m.Payload) + 1 + 16; len(body) != want {
		t.Errorf("body is %d bytes, want %d", len(body), want)
	}
	checkVAPID(t, got.Header.Get("Authorization"), key, srv.URL)

	status = http.StatusGone
	if err := Send(context.Background(), srv.Client(), sub, m, key, "https://broker.example"); err != ErrGone {
		t.Fatalf("Send to a gone subscription: %v", err)
	}
}

// checkVAPID verifies the JWT in auth against key, as a push service
// would.
func checkVAPID(t *testing.T, auth string, key *Key, audience string) {
	t.Helper()
	rest, ok := strings.CutPrefix(auth, "vapid t=")
	if !ok {
		t.Fatalf("Authorization %q", auth)
	}
	jwt, k, ok := strings.Cut(rest, ", k=")
	if !ok || k != key.Public() {
		t.Fatalf("Authorization %q", auth)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT %q", jwt)
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(mustB64(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != audience || claims.Sub != "https://broker.example" || claims.Exp < time.Now().Unix() {
		t.Errorf("claims %+v", claims)
	}
	sig := mustB64(t, parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.priv.PublicKey, digest[:], r, s) {
		t.Error("JWT signature does not verify")
	}
}
