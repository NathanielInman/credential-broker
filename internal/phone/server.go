// Package phone is broker's phone page: an HTTPS page on the local
// network where an enrolled phone shows a request and approves it with a
// security key plugged into the phone.
//
// The key never signs anything for broker. At enrollment the page makes a
// WebAuthn credential on it and asks for its PRF output (hmac-secret) on
// a random salt; that output derives an age X25519 identity (Identity),
// and the vault's private tier is wrapped to its recipient like any other
// unlock. Approving asks the key for the same output again, which needs a
// touch, and the page sends it back here over TLS. Whatever the page
// sends is checked against the recipient enrolled for that phone before
// it goes anywhere.
//
// The page only listens while a request or an enrollment is open.
package phone

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"filippo.io/age"

	"github.com/NathanielInman/credential-broker/internal/approval"
	"github.com/NathanielInman/credential-broker/internal/config"
)

//go:embed assets
var assets embed.FS

var b64 = base64.RawURLEncoding

const (
	// pollWait is how long a poll waits for news before answering empty.
	pollWait = 25 * time.Second
	// drainWait is how long Close waits for an open page to fetch the
	// outcome before the page stops listening.
	drainWait = 2 * time.Second
	// maxBody bounds a request body from the page.
	maxBody = 64 << 10
)

// Unlock is a phone's answer to a request: the identity its key's PRF
// output derives, already checked against the phone's recipient, and the
// grant duration the human picked.
type Unlock struct {
	Phone    string
	Identity age.Identity
	For      int64
}

// Enrollment is what the page sends back when a phone enrolls.
type Enrollment struct {
	// Credential is the WebAuthn credential ID, base64url.
	Credential string
	// Identity is what the key's PRF output on the enrollment salt
	// derives.
	Identity *age.X25519Identity
	// Token is the SHA-256 (hex) of the token the phone keeps to see
	// requests.
	Token string
	// Push is the phone's push subscription, or nil if it said no to
	// notifications.
	Push *config.PushSubscription
}

// Server is the phone page while it listens.
type Server struct {
	origin string
	rpID   string
	srv    *http.Server
	addr   net.Addr

	// Request mode: the phones that may see it, the session's messages,
	// and what comes back.
	phones  []config.Unlock
	replies chan approval.Reply
	unlocks chan Unlock

	// Enrollment mode.
	enroll *enrollment

	mu        sync.Mutex
	msgs      []json.RawMessage
	wake      chan struct{}
	closed    bool
	polled    bool
	delivered int
}

type enrollment struct {
	token  string
	salt   []byte
	vapid  string
	userID []byte
	done   chan Enrollment
	used   bool
	// phone is the SHA-256 of the token the enrolled phone got, which it
	// shows to learn how the PC side went.
	phone string
	// finished is set by Finish, with failure the error if any, and told
	// once the page has fetched it.
	finished bool
	failure  string
	told     bool
}

// Origin checks page's settings and returns the relying party ID (the
// origin's host) phones' keys are bound to.
func Origin(page config.PhonePage) (string, error) {
	u, err := url.Parse(page.Origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("phone page origin %q: want https://HOST[:PORT]", page.Origin)
	}
	if net.ParseIP(u.Hostname()) != nil {
		return "", fmt.Errorf("phone page origin %q: WebAuthn needs a domain name, not an IP address", page.Origin)
	}
	return u.Hostname(), nil
}

// StartRequest opens the page for one request, for the phones among
// unlocks. The Server is the request's approval.Remote.
func StartRequest(page config.PhonePage, unlocks []config.Unlock) (*Server, error) {
	s := &Server{
		replies: make(chan approval.Reply, 8),
		unlocks: make(chan Unlock, 1),
	}
	for _, u := range unlocks {
		if u.Phone != nil {
			s.phones = append(s.phones, u)
		}
	}
	if len(s.phones) == 0 {
		return nil, errors.New("no phone is enrolled")
	}
	return s, s.start(page)
}

// StartEnroll opens the page for enrolling one phone, with the key's PRF
// asked on salt and pushes subscribed to vapid (a VAPID public key). The
// phone opens URL.
func StartEnroll(page config.PhonePage, salt []byte, vapid string) (*Server, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	userID := make([]byte, 16)
	if _, err := rand.Read(userID); err != nil {
		return nil, err
	}
	s := &Server{enroll: &enrollment{token: token, salt: salt, vapid: vapid, userID: userID, done: make(chan Enrollment, 1)}}
	return s, s.start(page)
}

func (s *Server) start(page config.PhonePage) error {
	rpID, err := Origin(page)
	if err != nil {
		return err
	}
	s.origin, s.rpID = trimSlash(page.Origin), rpID
	s.wake = make(chan struct{})
	cert, err := tls.LoadX509KeyPair(page.Cert, page.Key)
	if err != nil {
		return fmt.Errorf("phone page certificate: %w", err)
	}
	ln, err := net.Listen("tcp", page.Listen)
	if err != nil {
		return fmt.Errorf("phone page: %w", err)
	}
	s.addr = ln.Addr()
	s.srv = &http.Server{
		Handler:           s.routes(),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
		// Handshake noise (a scanner on the network, a phone that gave
		// up) would otherwise land in the agent's stderr.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() { _ = s.srv.ServeTLS(ln, "", "") }()
	return nil
}

func trimSlash(origin string) string {
	for len(origin) > 0 && origin[len(origin)-1] == '/' {
		origin = origin[:len(origin)-1]
	}
	return origin
}

// Addr is where the page listens.
func (s *Server) Addr() net.Addr { return s.addr }

// EnrollURL is the address a phone opens to enroll. The token is in the
// fragment, which the browser never sends anywhere.
func (s *Server) EnrollURL() string { return s.origin + "/enroll#" + s.enroll.token }

// Enrolled delivers the phone's answer once it has enrolled.
func (s *Server) Enrolled() <-chan Enrollment { return s.enroll.done }

// Finish tells the enrolling page how the PC side went: err is why the
// vault was not wrapped to the phone's key, or nil once it is.
func (s *Server) Finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enroll == nil || s.enroll.finished {
		return
	}
	s.enroll.finished = true
	if err != nil {
		s.enroll.failure = err.Error()
	}
	s.wakeLocked()
}

// Unlocks delivers each phone's checked answer to the request.
func (s *Server) Unlocks() <-chan Unlock { return s.unlocks }

// Replies is the phones' other answers (deny, a shorter duration), for
// the session.
func (s *Server) Replies() <-chan approval.Reply { return s.replies }

// Write takes one message line from the session for the page to fetch.
func (s *Server) Write(p []byte) (int, error) {
	line := make([]byte, len(p))
	copy(line, p)
	for len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	s.mu.Lock()
	s.msgs = append(s.msgs, line)
	s.wakeLocked()
	s.mu.Unlock()
	return len(p), nil
}

func (s *Server) wakeLocked() {
	close(s.wake)
	s.wake = make(chan struct{})
}

// Close stops the page. A phone that has the request open gets a moment
// to fetch how it ended first.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	s.wakeLocked()
	polled := s.polled
	s.mu.Unlock()
	if polled {
		for end := time.Now().Add(drainWait); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			s.mu.Lock()
			caught := s.delivered >= len(s.msgs) && (s.enroll == nil || !s.enroll.finished || s.enroll.told)
			s.mu.Unlock()
			if caught {
				break
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.srv.Shutdown(ctx) != nil {
		_ = s.srv.Close()
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.static("assets/index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /enroll", s.static("assets/index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /app.js", s.static("assets/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /sw.js", s.static("assets/sw.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /icon.svg", s.static("assets/icon.svg", "image/svg+xml"))
	if s.enroll != nil {
		mux.HandleFunc("GET /api/enroll", s.enrollOptions)
		mux.HandleFunc("POST /api/enroll", s.enrollFinish)
		mux.HandleFunc("GET /api/enroll/result", s.enrollResult)
	} else {
		mux.HandleFunc("GET /api/poll", s.poll)
		mux.HandleFunc("POST /api/reply", s.reply)
		mux.HandleFunc("POST /api/unlock", s.unlock)
	}
	return headers(mux)
}

// headers sets what every response carries: a CSP that lets the page
// talk to nothing but this origin, no framing, no sniffing, no referrer.
func headers(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
			"connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "no-referrer")
		hd.Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func (s *Server) static(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := assets.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if name == "assets/sw.js" {
			w.Header().Set("Service-Worker-Allowed", "/")
		}
		_, _ = w.Write(data)
	}
}

// bearer is the token in r's Authorization header.
func bearer(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

// TokenHash is how a phone's page token is kept in the config.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// phone is the enrolled phone whose token r carries.
func (s *Server) phone(r *http.Request) (config.Unlock, bool) {
	tok := bearer(r)
	if tok == "" {
		return config.Unlock{}, false
	}
	h := TokenHash(tok)
	for _, u := range s.phones {
		if subtle.ConstantTimeCompare([]byte(h), []byte(u.Phone.Token)) == 1 {
			return u, true
		}
	}
	return config.Unlock{}, false
}

// sameOrigin checks a write came from the page itself.
func (s *Server) sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == s.origin
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "bad request body")
		return false
	}
	return true
}

// pollResponse is what the page gets each time it asks for news.
type pollResponse struct {
	Messages []json.RawMessage `json:"messages"`
	Next     int               `json:"next"`
	Closed   bool              `json:"closed"`
	// The phone's key, for navigator.credentials.get.
	RPID       string `json:"rpId"`
	Credential string `json:"credential"`
	Salt       string `json:"salt"`
}

// poll answers with the session's messages after ?after=N, waiting up to
// pollWait for one if there is none yet.
func (s *Server) poll(w http.ResponseWriter, r *http.Request) {
	u, ok := s.phone(r)
	if !ok {
		fail(w, http.StatusUnauthorized, "this phone is not enrolled")
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	timeout := time.NewTimer(pollWait)
	defer timeout.Stop()
	for {
		s.mu.Lock()
		s.polled = true
		after = min(max(after, 0), len(s.msgs))
		if after < len(s.msgs) || s.closed {
			res := pollResponse{
				Messages: append([]json.RawMessage{}, s.msgs[after:]...),
				Next:     len(s.msgs),
				Closed:   s.closed,
				RPID:     s.rpID, Credential: u.Phone.Credential, Salt: u.Phone.Salt,
			}
			s.delivered = max(s.delivered, res.Next)
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, res)
			return
		}
		wake := s.wake
		s.mu.Unlock()
		select {
		case <-wake:
		case <-timeout.C:
			writeJSON(w, http.StatusOK, pollResponse{Messages: []json.RawMessage{}, Next: after,
				RPID: s.rpID, Credential: u.Phone.Credential, Salt: u.Phone.Salt})
			return
		case <-r.Context().Done():
			return
		}
	}
}

// reply takes a deny or a shorter duration from the page.
func (s *Server) reply(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.phone(r); !ok || !s.sameOrigin(r) {
		fail(w, http.StatusUnauthorized, "this phone is not enrolled")
		return
	}
	var rep approval.Reply
	if !readJSON(w, r, &rep) {
		return
	}
	if rep.Type != "deny" && rep.Type != "for" {
		fail(w, http.StatusBadRequest, "unknown reply")
		return
	}
	select {
	case s.replies <- approval.Reply{Type: rep.Type, For: rep.For}:
	case <-r.Context().Done():
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unlock takes the key's PRF output. It must derive the recipient this
// phone enrolled; anything else is refused here and never reaches the
// vault.
func (s *Server) unlock(w http.ResponseWriter, r *http.Request) {
	u, ok := s.phone(r)
	if !ok || !s.sameOrigin(r) {
		fail(w, http.StatusUnauthorized, "this phone is not enrolled")
		return
	}
	var body struct {
		PRF string `json:"prf"`
		For int64  `json:"for"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	prf, err := b64.DecodeString(body.PRF)
	if err != nil {
		fail(w, http.StatusBadRequest, "bad PRF output")
		return
	}
	id, err := Identity(prf)
	clear(prf)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if id.Recipient().String() != u.Recipient {
		fail(w, http.StatusForbidden, "that is not the security key enrolled for this phone")
		return
	}
	select {
	case s.unlocks <- Unlock{Phone: u.Phone.Name, Identity: id, For: body.For}:
		w.WriteHeader(http.StatusNoContent)
	default:
		fail(w, http.StatusConflict, "the request is already being unlocked")
	}
}

// enrollOptions gives an enrolling page what it needs for WebAuthn and
// push, once it shows the enrollment token.
func (s *Server) enrollOptions(w http.ResponseWriter, r *http.Request) {
	if !s.enrollToken(r) {
		fail(w, http.StatusUnauthorized, "this enrollment link is not valid or was already used; run broker phone enroll again")
		return
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"rpId":      s.rpID,
		"userId":    b64.EncodeToString(s.enroll.userID),
		"challenge": b64.EncodeToString(challenge),
		"salt":      b64.EncodeToString(s.enroll.salt),
		"vapid":     s.enroll.vapid,
	})
}

func (s *Server) enrollToken(r *http.Request) bool {
	s.mu.Lock()
	used := s.enroll.used
	s.mu.Unlock()
	return !used && subtle.ConstantTimeCompare([]byte(bearer(r)), []byte(s.enroll.token)) == 1
}

// enrollFinish takes the new credential, its PRF output, and the push
// subscription, and gives the phone the token it shows from now on.
func (s *Server) enrollFinish(w http.ResponseWriter, r *http.Request) {
	if !s.enrollToken(r) || !s.sameOrigin(r) {
		fail(w, http.StatusUnauthorized, "this enrollment link is not valid or was already used; run broker phone enroll again")
		return
	}
	var body struct {
		Credential string                   `json:"credential"`
		PRF        string                   `json:"prf"`
		Push       *config.PushSubscription `json:"push"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if cred, err := b64.DecodeString(body.Credential); err != nil || len(cred) == 0 || len(cred) > 1023 {
		fail(w, http.StatusBadRequest, "bad credential ID")
		return
	}
	prf, err := b64.DecodeString(body.PRF)
	if err != nil {
		fail(w, http.StatusBadRequest, "bad PRF output")
		return
	}
	id, err := Identity(prf)
	clear(prf)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if p := body.Push; p != nil && (p.Endpoint == "" || p.P256dh == "" || p.Auth == "") {
		body.Push = nil
	}
	token, err := randomToken()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.mu.Lock()
	if s.enroll.used {
		s.mu.Unlock()
		fail(w, http.StatusConflict, "this phone has already enrolled")
		return
	}
	s.enroll.used = true
	s.enroll.phone = TokenHash(token)
	s.mu.Unlock()
	s.enroll.done <- Enrollment{Credential: body.Credential, Identity: id, Token: TokenHash(token), Push: body.Push}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// enrollResult tells the enrolled phone how the PC side went, waiting up
// to pollWait for it: the phone's part is only half of enrolling.
func (s *Server) enrollResult(w http.ResponseWriter, r *http.Request) {
	tok := bearer(r)
	timeout := time.NewTimer(pollWait)
	defer timeout.Stop()
	for {
		s.mu.Lock()
		en := s.enroll
		if en.phone == "" || tok == "" || subtle.ConstantTimeCompare([]byte(TokenHash(tok)), []byte(en.phone)) != 1 {
			s.mu.Unlock()
			fail(w, http.StatusUnauthorized, "this phone has not enrolled here")
			return
		}
		s.polled = true
		if en.finished {
			en.told = true
			res := map[string]any{"done": true, "error": en.failure}
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, res)
			return
		}
		wake := s.wake
		s.mu.Unlock()
		select {
		case <-wake:
		case <-timeout.C:
			writeJSON(w, http.StatusOK, map[string]any{"done": false})
			return
		case <-r.Context().Done():
			return
		}
	}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b64.EncodeToString(b), nil
}
