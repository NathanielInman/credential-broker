package phone

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/NathanielInman/credential-broker/internal/approval"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/phone/phonetest"
)

func prf(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestIdentity(t *testing.T) {
	a, err := Identity(prf(1))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Identity(prf(1))
	other, _ := Identity(prf(2))
	if a.String() != again.String() || a.String() == other.String() {
		t.Fatal("Identity is not a function of the PRF output")
	}
	// It is a plain age identity: the stock code wraps to its recipient
	// and unwraps with it.
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, a.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, "file key")
	w.Close()
	r, err := age.Decrypt(&buf, again)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); string(got) != "file key" {
		t.Fatalf("decrypted %q", got)
	}
	if _, err := Identity(prf(1)[:16]); err == nil {
		t.Fatal("Identity took a short PRF output")
	}
}

func TestBech32(t *testing.T) {
	// BIP 173's valid test vector with an empty data part.
	if got := bech32("a", nil); got != "a12uel5l" {
		t.Fatalf("bech32 = %q", got)
	}
}

// page is the test page's settings: its origin, served at addr.
func page(t *testing.T) (config.PhonePage, *http.Client) {
	cert, key, pool := phonetest.Cert(t)
	addr := phonetest.FreeAddr(t)
	_, port, _ := strings.Cut(addr, ":")
	return config.PhonePage{Origin: "https://" + phonetest.Host + ":" + port, Listen: addr, Cert: cert, Key: key},
		phonetest.Client(pool, addr)
}

type client struct {
	t      *testing.T
	http   *http.Client
	origin string
	token  string
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rd = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.origin+path, rd)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Origin", c.origin)
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestRequest(t *testing.T) {
	pg, hc := page(t)
	id, _ := Identity(prf(7))
	phones := []config.Unlock{
		{Path: "/key.txt", Recipient: "age1unused"},
		{Recipient: id.Recipient().String(), Phone: &config.PhoneKey{Name: "pixel", Credential: "Y3JlZA", Salt: "c2FsdA", Token: TokenHash("tok")}},
	}
	s, err := StartRequest(pg, phones)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := &client{t: t, http: hc, origin: pg.Origin}

	// The page itself needs no token; the request does.
	res, err := hc.Get(pg.Origin + "/")
	if err != nil || res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("GET /: %v %v", res, err)
	}
	if code, _ := c.do("GET", "/api/poll", nil); code != http.StatusUnauthorized {
		t.Fatalf("poll without a token: %d", code)
	}
	c.token = "tok"

	s.Write([]byte(`{"type":"request","request":{"secrets":[{"name":"API_TOKEN"}]}}` + "\n"))
	code, poll := c.do("GET", "/api/poll?after=0", nil)
	if code != 200 || poll["next"] != float64(1) || poll["credential"] != "Y3JlZA" || poll["salt"] != "c2FsdA" || poll["rpId"] != phonetest.Host {
		t.Fatalf("poll: %d %v", code, poll)
	}
	if msgs := poll["messages"].([]any); len(msgs) != 1 || msgs[0].(map[string]any)["type"] != "request" {
		t.Fatalf("messages %v", msgs)
	}
	// A poll with nothing new waits for the next message.
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Write([]byte(`{"type":"status","text":"x"}`))
	}()
	if _, poll := c.do("GET", "/api/poll?after=1", nil); poll["next"] != float64(2) {
		t.Fatalf("long poll: %v", poll)
	}

	if code, _ := c.do("POST", "/api/reply", approval.Reply{Type: "for", For: 300}); code != http.StatusNoContent {
		t.Fatalf("reply: %d", code)
	}
	if rep := <-s.Replies(); rep.Type != "for" || rep.For != 300 {
		t.Fatalf("reply %+v", rep)
	}
	if code, _ := c.do("POST", "/api/reply", approval.Reply{Type: "approve"}); code != http.StatusBadRequest {
		t.Fatalf("an approve reply must come with the key: %d", code)
	}

	// Another key's output is refused before it goes anywhere.
	if code, out := c.do("POST", "/api/unlock", map[string]any{"prf": b64.EncodeToString(prf(8)), "for": 300}); code != http.StatusForbidden {
		t.Fatalf("unlock with the wrong key: %d %v", code, out)
	}
	select {
	case u := <-s.Unlocks():
		t.Fatalf("the wrong key reached Unlocks: %+v", u)
	default:
	}
	// A write from another origin is refused.
	c.origin, c.http = pg.Origin, hc
	req, _ := http.NewRequest("POST", pg.Origin+"/api/unlock", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Origin", "https://evil.example")
	if res, err := hc.Do(req); err != nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-origin unlock: %v %v", res, err)
	}

	if code, _ := c.do("POST", "/api/unlock", map[string]any{"prf": b64.EncodeToString(prf(7)), "for": 300}); code != http.StatusNoContent {
		t.Fatalf("unlock: %d", code)
	}
	u := <-s.Unlocks()
	if u.Phone != "pixel" || u.For != 300 || u.Identity.(*age.X25519Identity).String() != id.String() {
		t.Fatalf("unlock %+v", u)
	}

	// Close lets the open page fetch the outcome first.
	s.Write([]byte(`{"type":"done","text":"Granted for 5m"}`))
	go s.Close()
	_, poll = c.do("GET", "/api/poll?after=2", nil)
	if msgs := poll["messages"].([]any); len(msgs) != 1 || msgs[0].(map[string]any)["type"] != "done" {
		t.Fatalf("after close: %v", poll)
	}
}

func TestEnroll(t *testing.T) {
	pg, hc := page(t)
	s, err := StartEnroll(pg, prf(3), "vapid-public")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, token, ok := strings.Cut(s.EnrollURL(), "/enroll#")
	if !ok || !strings.HasPrefix(s.EnrollURL(), pg.Origin) {
		t.Fatalf("enroll URL %q", s.EnrollURL())
	}
	c := &client{t: t, http: hc, origin: pg.Origin, token: "wrong"}
	if code, _ := c.do("GET", "/api/enroll", nil); code != http.StatusUnauthorized {
		t.Fatalf("options with a wrong token: %d", code)
	}
	c.token = token
	code, opts := c.do("GET", "/api/enroll", nil)
	if code != 200 || opts["rpId"] != phonetest.Host || opts["salt"] != b64.EncodeToString(prf(3)) || opts["vapid"] != "vapid-public" {
		t.Fatalf("options: %d %v", code, opts)
	}
	// Request mode's API is not there while enrolling.
	if code, _ := c.do("GET", "/api/poll", nil); code != http.StatusNotFound {
		t.Fatalf("poll while enrolling: %d", code)
	}
	body := map[string]any{
		"credential": "Y3JlZA",
		"prf":        b64.EncodeToString(prf(4)),
		"push":       map[string]string{"endpoint": "https://push.example/x", "p256dh": "p", "auth": "a"},
	}
	code, out := c.do("POST", "/api/enroll", body)
	if code != 200 || out["token"] == "" {
		t.Fatalf("enroll: %d %v", code, out)
	}
	en := <-s.Enrolled()
	want, _ := Identity(prf(4))
	if en.Credential != "Y3JlZA" || en.Identity.String() != want.String() || en.Token != TokenHash(out["token"].(string)) ||
		en.Push == nil || en.Push.Endpoint != "https://push.example/x" {
		t.Fatalf("enrollment %+v", en)
	}
	// The link works once.
	if code, _ := c.do("POST", "/api/enroll", body); code != http.StatusUnauthorized {
		t.Fatalf("second enroll: %d", code)
	}
}

func TestOrigin(t *testing.T) {
	for origin, ok := range map[string]bool{
		"https://broker.twinspear.com:8443": true,
		"https://broker.twinspear.com/":     true,
		"http://broker.twinspear.com":       false,
		"https://192.168.1.55:8443":         false,
		"https://broker.twinspear.com/x":    false,
	} {
		if _, err := Origin(config.PhonePage{Origin: origin}); (err == nil) != ok {
			t.Errorf("Origin(%s): %v", origin, err)
		}
	}
}
