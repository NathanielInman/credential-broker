package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	brokercrypto "github.com/nate/credential-broker/crypto"
	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// POST /authSecure - Diffie-Hellman key exchange
func (s *Server) handleAuthSecure(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	id := r.Header.Get("id")
	key := r.Header.Get("key")
	prime := r.Header.Get("prime")

	if key == "" {
		internal.Log(ip, "", "authSecure: Key missing", true)
		http.Error(w, "User key was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if id == "" {
		internal.Log(ip, "", "authSecure: Id missing", true)
		http.Error(w, "User session id was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if prime == "" {
		internal.Log(ip, "", "authSecure: Prime missing", true)
		http.Error(w, "Diffie hellman prime missing in the header.", http.StatusUnauthorized)
		return
	}

	serverPub, secret, err := brokercrypto.DHFromPrime(prime, key)
	if err != nil {
		internal.Log(ip, id, "authSecure: DH computation failed", true)
		http.Error(w, "DH computation failed.", http.StatusInternalServerError)
		return
	}

	internal.Log(ip, id, "Auth Secure", false)

	session := map[string]interface{}{
		"secret":        secret,
		"authenticated": false,
	}
	sessionBytes, _ := json.Marshal(session)
	ttl := time.Duration(s.Broker.SessionTTL) * time.Millisecond
	s.DB.Set("sessions", id, sessionBytes, ttl)

	// Return server's public key as JSON (matching Node.js: res.status(200).json(serverKey))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverPub)
}

// POST /authIdentify - Send encrypted challenge
func (s *Server) handleAuthIdentify(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	id := r.Header.Get("id")

	if id == "" {
		internal.Log(ip, "", "authIdentify: Id missing", true)
		http.Error(w, "User session id was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("name") == "" {
		internal.Log(ip, "", "authIdentify: Name missing", true)
		http.Error(w, "Encrypted name missing from the header.", http.StatusUnauthorized)
		return
	}

	sessionData, err := s.DB.Get("sessions", id)
	if err != nil || sessionData == nil {
		internal.Log(ip, id, "authIdentify: No session to attach", true)
		http.Error(w, "Session expired.", http.StatusUnauthorized)
		return
	}

	var session map[string]interface{}
	json.Unmarshal(sessionData, &session)
	secret := session["secret"].(string)

	name, err := brokercrypto.AESDecrypt(r.Header.Get("name"), secret)
	if err != nil {
		http.Error(w, "Decryption failed.", http.StatusUnauthorized)
		return
	}

	userData, err := s.DB.Get("users", name)
	if err != nil || userData == nil {
		http.Error(w, "User not found.", http.StatusUnauthorized)
		return
	}

	var user model.User
	json.Unmarshal(userData, &user)

	// Generate random challenge
	challenge, err := brokercrypto.RandomBase64(256)
	if err != nil {
		http.Error(w, "Server error.", http.StatusInternalServerError)
		return
	}

	// Update session with challenge
	session["challenge"] = challenge
	sessionBytes, _ := json.Marshal(session)
	ttl := time.Duration(s.Broker.SessionTTL) * time.Millisecond
	s.DB.Set("sessions", id, sessionBytes, ttl)

	// Respond: PGP encrypt the challenge, then AES encrypt
	pgpEncrypted, err := brokercrypto.Encrypt(fmt.Sprintf("%q", challenge), user.Key)
	if err != nil {
		http.Error(w, "Encryption failed.", http.StatusInternalServerError)
		return
	}

	aesEncrypted := brokercrypto.AESEncrypt(pgpEncrypted, secret)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(aesEncrypted))
}

// POST /authChallenge - Verify challenge response
func (s *Server) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	id := r.Header.Get("id")

	if id == "" {
		internal.Log(ip, "", "authChallenge: Id missing", true)
		http.Error(w, "User session id was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("name") == "" {
		internal.Log(ip, "", "authChallenge: Name missing", true)
		http.Error(w, "Encrypted name missing from the header.", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("challenge") == "" {
		internal.Log(ip, "", "authChallenge: Challenge missing", true)
		http.Error(w, "Encrypted challenge missing from the header.", http.StatusUnauthorized)
		return
	}

	sessionData, err := s.DB.Get("sessions", id)
	if err != nil || sessionData == nil {
		internal.Log(ip, id, "authChallenge: No session to attach", true)
		http.Error(w, "Session expired.", http.StatusUnauthorized)
		return
	}

	var session map[string]interface{}
	json.Unmarshal(sessionData, &session)
	secret := session["secret"].(string)
	challenge := session["challenge"].(string)

	name, _ := brokercrypto.AESDecrypt(r.Header.Get("name"), secret)
	challengeResponse, _ := brokercrypto.AESDecrypt(r.Header.Get("challenge"), secret)

	userData, _ := s.DB.Get("users", name)
	var user model.User
	json.Unmarshal(userData, &user)

	expectedHash := brokercrypto.MD5(challenge)

	if challengeResponse != expectedHash {
		internal.Log(ip, id, "authChallenge: Failure to match", true)
		http.Error(w, "Challenge does not match", http.StatusUnauthorized)
		return
	}

	// Mark session as authenticated
	newSession := map[string]interface{}{
		"secret":        secret,
		"authenticated": true,
	}
	sessionBytes, _ := json.Marshal(newSession)
	ttl := time.Duration(s.Broker.SessionTTL) * time.Millisecond
	s.DB.Set("sessions", id, sessionBytes, ttl)

	// Respond with encrypted "Authenticated"
	pgpEncrypted, _ := brokercrypto.Encrypt(fmt.Sprintf("%q", "Authenticated"), user.Key)
	aesEncrypted := brokercrypto.AESEncrypt(pgpEncrypted, secret)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(aesEncrypted))
}

// POST /auth2FAInitialize - Generate TOTP secret
func (s *Server) handleAuth2FAInitialize(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	id := r.Header.Get("id")

	if id == "" {
		internal.Log(ip, "", "auth2FAInitialize: Id missing", true)
		http.Error(w, "User session id was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("name") == "" {
		internal.Log(ip, "", "auth2FAInitialize: Name missing", true)
		http.Error(w, "Encrypted name missing from the header.", http.StatusUnauthorized)
		return
	}

	sessionData, err := s.DB.Get("sessions", id)
	if err != nil || sessionData == nil {
		internal.Log(ip, id, "auth2FAInitialize: No session to attach", true)
		http.Error(w, "Session expired.", http.StatusUnauthorized)
		return
	}

	var session map[string]interface{}
	json.Unmarshal(sessionData, &session)
	secret := session["secret"].(string)

	name, _ := brokercrypto.AESDecrypt(r.Header.Get("name"), secret)

	userData, _ := s.DB.Get("users", name)
	var user model.User
	json.Unmarshal(userData, &user)

	// Generate TOTP secret
	totpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "CredentialBroker",
		AccountName: name,
	})
	if err != nil {
		http.Error(w, "Failed to generate 2FA secret.", http.StatusInternalServerError)
		return
	}

	// Save secret to user
	user.TwoFactorSecret = totpKey.Secret()
	userBytes, _ := json.Marshal(user)
	s.DB.Set("users", name, userBytes, 0)

	// Respond with OTP URL
	body := map[string]string{"url": totpKey.URL()}
	internal.RespondWith(w, secret, user.Key, http.StatusOK, body)
}

// POST /auth2FAValidate - Validate TOTP token
func (s *Server) handleAuth2FAValidate(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	id := r.Header.Get("id")

	if id == "" {
		internal.Log(ip, "", "auth2FAValidate: Id missing", true)
		http.Error(w, "User session id was not passed in the header.", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("name") == "" {
		internal.Log(ip, "", "auth2FAValidate: Name missing", true)
		http.Error(w, "Encrypted name missing from the header.", http.StatusUnauthorized)
		return
	}

	sessionData, err := s.DB.Get("sessions", id)
	if err != nil || sessionData == nil {
		internal.Log(ip, id, "auth2FAValidate: No session to attach", true)
		http.Error(w, "Session expired.", http.StatusUnauthorized)
		return
	}

	var session map[string]interface{}
	json.Unmarshal(sessionData, &session)
	secret := session["secret"].(string)

	name, _ := brokercrypto.AESDecrypt(r.Header.Get("name"), secret)

	userData, _ := s.DB.Get("users", name)
	var user model.User
	json.Unmarshal(userData, &user)

	// The body is a signed token - verify it
	body := internal.GetBody(r.Context())
	token, err := brokercrypto.Verify(body, user.Key)
	if err != nil {
		http.Error(w, "Verification failed.", http.StatusUnauthorized)
		return
	}

	valid := totp.Validate(token, user.TwoFactorSecret)

	opts := totp.ValidateOpts{
		Period:    30,
		Skew:     1,
		Digits:   otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
	_ = opts // use default Validate which has similar behavior

	if valid {
		internal.Log(ip, user.Name, "auth2FAValidate: Successfully setup 2FA", false)
		user.TwoFactorActive = true
		userBytes, _ := json.Marshal(user)
		s.DB.Set("users", name, userBytes, 0)
	}

	result := map[string]bool{"valid": valid}
	internal.RespondWith(w, secret, user.Key, http.StatusOK, result)
}
