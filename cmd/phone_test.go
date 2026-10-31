package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/phone"
	"github.com/NathanielInman/credential-broker/internal/phone/phonetest"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

var b64url = base64.RawURLEncoding

// testPhone acts as a phone's page against the phone page at origin.
type testPhone struct {
	t      *testing.T
	http   *http.Client
	origin string
	token  string
}

func (p *testPhone) call(method, path string, body any) (int, map[string]any, error) {
	var rd io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rd = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, p.origin+path, rd)
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Origin", p.origin)
	res, err := p.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out, nil
}

// poll asks for messages after n until one of type want arrives, retrying
// while the page is not listening yet.
func (p *testPhone) poll(after *int, want string) map[string]any {
	p.t.Helper()
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end); {
		code, out, err := p.call("GET", "/api/poll?after="+strconv.Itoa(*after), nil)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if code != 200 {
			p.t.Fatalf("poll: %d %v", code, out)
		}
		*after = int(out["next"].(float64))
		for _, m := range out["messages"].([]any) {
			if m := m.(map[string]any); m["type"] == want {
				return m
			}
		}
	}
	p.t.Fatalf("no %s message", want)
	return nil
}

// pushServer is a push service that counts what it gets.
func pushServer(t *testing.T) (*httptest.Server, chan *http.Request) {
	got := make(chan *http.Request, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		got <- r
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func subscription(t *testing.T, endpoint string) map[string]string {
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	return map[string]string{"endpoint": endpoint, "p256dh": b64url.EncodeToString(ua.PublicKey().Bytes()), "auth": b64url.EncodeToString(auth)}
}

func TestPhoneEnrollRequestRm(t *testing.T) {
	grantVault(t)
	hardenSelf = func() error { return nil }
	origDesktop, origLaunch := desktopWindow, launchApprover
	t.Cleanup(func() { hardenSelf, desktopWindow, launchApprover = harden, origDesktop, origLaunch })

	certFile, keyFile, pool := phonetest.Cert(t)
	addr := phonetest.FreeAddr(t)
	origin := "https://" + phonetest.Host + ":" + addr[strings.LastIndexByte(addr, ':')+1:]
	push, pushes := pushServer(t)
	ph := &testPhone{t: t, http: phonetest.Client(pool, addr), origin: origin}
	prf := bytes.Repeat([]byte{9}, 32)

	// Enroll: broker prints the link, the phone follows it.
	outR, outW := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run("test", []string{"phone", "enroll", "--name", "pixel", "--origin", origin,
			"--cert", certFile, "--key", keyFile, "--listen", addr}, strings.NewReader(""), outW, &stderr)
		outW.Close()
	}()
	var out strings.Builder
	sc := bufio.NewScanner(outR)
	for sc.Scan() {
		out.WriteString(sc.Text() + "\n")
		if _, tok, ok := strings.Cut(sc.Text(), "/enroll#"); ok {
			ph.token = tok
			break
		}
	}
	if ph.token == "" {
		t.Fatalf("no enroll link in %q (stderr %q)", out.String(), stderr.String())
	}
	readAll := make(chan struct{})
	go func() {
		for sc.Scan() {
			out.WriteString(sc.Text() + "\n")
		}
		close(readAll)
	}()
	code, opts, err := ph.call("GET", "/api/enroll", nil)
	if err != nil || code != 200 {
		t.Fatalf("enroll options: %d %v %v", code, opts, err)
	}
	code, res, err := ph.call("POST", "/api/enroll", map[string]any{
		"credential": "Y3JlZA", "prf": b64url.EncodeToString(prf), "push": subscription(t, push.URL+"/sub"),
	})
	if err != nil || code != 200 {
		t.Fatalf("enroll: %d %v %v", code, res, err)
	}
	// From now on the phone shows the token enrolling gave it, first to
	// hear that the PC side worked.
	ph.token = res["token"].(string)
	if code, res, err := ph.call("GET", "/api/enroll/result", nil); err != nil || code != 200 || res["done"] != true || res["error"] != "" {
		t.Fatalf("enroll result: %d %v %v", code, res, err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("enroll exit %d: %s", code, stderr.String())
	}
	<-readAll
	if s := out.String(); !strings.Contains(s, "enrolled phone pixel") || !strings.Contains(s, "sent a test notification") ||
		!strings.Contains(s, "phone pixel (hardware)") {
		t.Fatalf("enroll output %q", s)
	}
	if r := <-pushes; r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") {
		t.Fatalf("test push headers %v", r.Header)
	}
	if out, _ := mustRun(t, "phone", "ls"); !strings.Contains(out, "pixel (notifications on)") || !strings.Contains(out, origin) {
		t.Fatalf("phone ls %q", out)
	}
	if _, stderr, code := runStdin(t, "", "phone", "enroll", "--name", "pixel"); code != 64 || !strings.Contains(stderr, "a phone named pixel") {
		t.Fatalf("enrolling pixel twice: %d %s", code, stderr)
	}

	// A request with no desktop goes to the phone alone.
	desktopWindow = func() error { return errors.New("no desktop in this test") }
	launchApprover = func(ctx context.Context, req []byte, stderr io.Writer) ([]byte, error) {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cmd := exec.CommandContext(ctx, exe, "approve")
		cmd.Env = append(os.Environ(), envBroker+"=1")
		cmd.Stdin = bytes.NewReader(req)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, stderr
		err = cmd.Run()
		return out.Bytes(), err
	}
	type result struct {
		out, err string
		code     int
	}
	reqDone := make(chan result, 1)
	go func() {
		var o, e bytes.Buffer
		c := run("test", []string{"request", "API_TOKEN", "--for", "1h", "--reason", "from the couch"}, strings.NewReader(""), &o, &e)
		reqDone <- result{o.String(), e.String(), c}
	}()
	after := 0
	m := ph.poll(&after, "request")
	shownReq := m["request"].(map[string]any)
	if shownReq["reason"] != "from the couch" || shownReq["secrets"].([]any)[0].(map[string]any)["name"] != "API_TOKEN" {
		t.Fatalf("phone shown %v", shownReq)
	}
	select {
	case r := <-pushes:
		if r.Header.Get("Urgency") != "high" || r.Header.Get("TTL") == "0" {
			t.Fatalf("request push headers %v", r.Header)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no notification for the request")
	}
	// The wrong key is turned away; the enrolled one grants, for the
	// duration the phone picked.
	if code, _, _ := ph.call("POST", "/api/unlock", map[string]any{"prf": b64url.EncodeToString(bytes.Repeat([]byte{1}, 32)), "for": 900}); code != http.StatusForbidden {
		t.Fatalf("unlock with another key: %d", code)
	}
	if code, out, err := ph.call("POST", "/api/unlock", map[string]any{"prf": b64url.EncodeToString(prf), "for": 900}); err != nil || code != http.StatusNoContent {
		t.Fatalf("unlock: %d %v %v", code, out, err)
	}
	if m := ph.poll(&after, "done"); m["text"] != "Granted for 15m" || m["error"] == true {
		t.Fatalf("phone saw %v", m)
	}
	r := <-reqDone
	if r.code != 0 || !strings.HasPrefix(r.out, "granted API_TOKEN for 15m ") {
		t.Fatalf("request: exit %d, %q, %q", r.code, r.out, r.err)
	}

	// Removing the phone rewraps the vault without its key.
	p, _ := paths.Resolve()
	mustRun(t, "phone", "rm", "pixel")
	v, err := vault.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Config().Phones()) != 0 {
		t.Fatalf("phones after rm: %+v", v.Config().Phones())
	}
	id, _ := phone.Identity(prf)
	if _, err := v.UnlockWith(id); err == nil {
		t.Fatal("the removed phone's key still opens the vault")
	}
	if _, err := v.UnlockPrivate(nil); err != nil {
		t.Fatalf("the vault's own unlock after rm: %v", err)
	}
	if _, stderr, code := runStdin(t, "", "phone", "rm", "pixel"); code != 64 || !strings.Contains(stderr, "no phone named pixel") {
		t.Fatalf("rm twice: %d %s", code, stderr)
	}
	var cfg config.Config
	data, _ := os.ReadFile(p.ConfigFile())
	json.Unmarshal(data, &cfg)
	if cfg.Phone == nil || cfg.Phone.Origin != origin {
		t.Fatalf("page settings after rm: %+v", cfg.Phone)
	}
}

// enrollDirect enrolls a phone whose key gives prf straight into the
// vault, without the enroll command, and returns a phone for its page.
func enrollDirect(t *testing.T, prf []byte) *testPhone {
	t.Helper()
	certFile, keyFile, pool := phonetest.Cert(t)
	addr := phonetest.FreeAddr(t)
	origin := "https://" + phonetest.Host + ":" + addr[strings.LastIndexByte(addr, ':')+1:]
	p, _ := paths.Resolve()
	v, err := vault.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := v.UnlockPrivate(nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := phone.Identity(prf)
	v.Config().Phone = &config.PhonePage{Origin: origin, Listen: addr, Cert: certFile, Key: keyFile}
	unlocks := append(v.Config().Unlock, config.Unlock{Recipient: id.Recipient().String(), Phone: &config.PhoneKey{
		Name: "pixel", Credential: "Y3JlZA", Salt: "c2FsdA", Token: phone.TokenHash("tok"),
	}})
	if err := v.SetUnlocks(unlocks, priv, nil); err != nil {
		t.Fatal(err)
	}
	return &testPhone{t: t, http: phonetest.Client(pool, addr), origin: origin, token: "tok"}
}

// TestPhoneBesideWindow has the request in the window and on the phone at
// once; the phone answers first and the window hears how it ended.
func TestPhoneBesideWindow(t *testing.T) {
	grantVault(t)
	prf := bytes.Repeat([]byte{5}, 32)
	ph := enrollDirect(t, prf)
	log := useWindow(t, "wait")
	reqDone := make(chan int, 1)
	go func() {
		var o, e bytes.Buffer
		reqDone <- run("test", []string{"request", "API_TOKEN"}, strings.NewReader(""), &o, &e)
	}()
	after := 0
	ph.poll(&after, "request")
	if code, out, err := ph.call("POST", "/api/unlock", map[string]any{"prf": b64url.EncodeToString(prf), "for": 3600}); err != nil || code != http.StatusNoContent {
		t.Fatalf("unlock: %d %v %v", code, out, err)
	}
	if m := ph.poll(&after, "done"); m["text"] != "Granted for 1h" {
		t.Fatalf("phone saw %v", m)
	}
	if code := <-reqDone; code != 0 {
		t.Fatalf("request exit %d", code)
	}
	if _, msgs := shown(t, log); msgs[len(msgs)-1].Type != "done" || msgs[len(msgs)-1].Text != "Granted for 1h" {
		t.Fatalf("window ended with %+v", msgs[len(msgs)-1])
	}
	if _, ok := granted(t)["API_TOKEN"]; !ok {
		t.Fatal("API_TOKEN not granted")
	}
}

// TestPhoneDeny: Deny on the phone ends the request as a deny.
func TestPhoneDeny(t *testing.T) {
	grantVault(t)
	ph := enrollDirect(t, bytes.Repeat([]byte{6}, 32))
	// No window script: the approver has no window, only the phone.
	useWindow(t, "")
	reqDone := make(chan string, 1)
	go func() {
		var o, e bytes.Buffer
		run("test", []string{"request", "API_TOKEN"}, strings.NewReader(""), &o, &e)
		reqDone <- e.String()
	}()
	after := 0
	ph.poll(&after, "request")
	if code, _, err := ph.call("POST", "/api/reply", map[string]any{"type": "deny"}); err != nil || code != http.StatusNoContent {
		t.Fatalf("deny: %d %v", code, err)
	}
	if m := ph.poll(&after, "done"); m["text"] != "Denied" {
		t.Fatalf("phone saw %v", m)
	}
	if e := <-reqDone; !strings.Contains(e, `"error":"denied"`) {
		t.Fatalf("request stderr %q", e)
	}
}

func TestUnlockHint(t *testing.T) {
	file := filepath.Join(t.TempDir(), "yubikey-identity.txt")
	os.WriteFile(file, []byte("#       Serial: 39616611, Slot: 2\n#    Recipient: age1yubikey1q\nAGE-PLUGIN-YUBIKEY-1QQQ\n"), 0o600)
	cfg := &config.Config{Unlock: []config.Unlock{{Path: file, Identity: "AGE-PLUGIN-YUBIKEY-1QQQ"}}}
	if h := unlockHint(cfg); !strings.Contains(h, "serial 39616611") || !strings.Contains(h, "a different key can stay in the phone") {
		t.Fatalf("hint %q", h)
	}
	if h := unlockHint(&config.Config{Unlock: []config.Unlock{{Path: "/no/such/file", Passphrase: true}}}); strings.Contains(h, "serial") {
		t.Fatalf("hint without a YubiKey %q", h)
	}
}
