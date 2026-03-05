package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	brokercrypto "github.com/nate/credential-broker/crypto"
	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
	"github.com/pquerna/otp/totp"
)

// authenticated wraps a handler with the 4-layer auth middleware
func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		// Layer 1: Check session exists
		id := r.Header.Get("id")
		if id == "" {
			internal.Log(ip, "", fmt.Sprintf("%s: Key missing", r.URL.Path), true)
			http.Error(w, "User key was not passed in the header.", http.StatusUnauthorized)
			return
		}

		nameHeader := r.Header.Get("name")
		if nameHeader == "" {
			internal.Log(ip, id, fmt.Sprintf("%s: Username missing", r.URL.Path), true)
			http.Error(w, "User name was not passed in the header.", http.StatusUnauthorized)
			return
		}

		sessionData, err := s.DB.Get("sessions", id)
		if err != nil || sessionData == nil {
			http.Error(w, "Session expired.", http.StatusUnauthorized)
			return
		}

		var session struct {
			Secret        string `json:"secret"`
			Authenticated bool   `json:"authenticated"`
			Challenge     string `json:"challenge,omitempty"`
		}
		if err := json.Unmarshal(sessionData, &session); err != nil {
			internal.Log(ip, id, fmt.Sprintf("%s: Authentication Failure", r.URL.Path), true)
			http.Error(w, "Improper authentication headers.", http.StatusUnauthorized)
			return
		}

		if !session.Authenticated {
			http.Error(w, "Session expired.", http.StatusUnauthorized)
			return
		}

		// Decrypt name and email from headers
		name, err := brokercrypto.AESDecrypt(nameHeader, session.Secret)
		if err != nil {
			internal.Log(ip, id, fmt.Sprintf("%s: Authentication Failure", r.URL.Path), true)
			http.Error(w, "Improper authentication headers.", http.StatusUnauthorized)
			return
		}

		emailHeader := r.Header.Get("email")
		var email string
		if emailHeader != "" {
			email, _ = brokercrypto.AESDecrypt(emailHeader, session.Secret)
		}

		// Look up user
		userData, err := s.DB.Get("users", name)
		if err != nil || userData == nil {
			internal.Log(ip, name, fmt.Sprintf("%s: User does not exist", r.URL.Path), true)
			http.Error(w, "User does not exist.", http.StatusUnauthorized)
			return
		}

		var user model.User
		if err := json.Unmarshal(userData, &user); err != nil {
			internal.Log(ip, name, fmt.Sprintf("%s: User data corrupt", r.URL.Path), true)
			http.Error(w, "User data corrupted.", http.StatusUnauthorized)
			return
		}

		// Layer 4: Check 2FA
		twoFactorData, _ := s.DB.Get("twofactor", id)
		twoFactorToken := r.Header.Get("Two-Factor-Token")

		if twoFactorData == nil && twoFactorToken == "" {
			http.Error(w, "Two-factor expired.", http.StatusUnauthorized)
			return
		} else if twoFactorData == nil && twoFactorToken != "" {
			valid := totp.Validate(twoFactorToken, user.TwoFactorSecret)
			if !valid {
				internal.Log(ip, name, fmt.Sprintf("%s: Invalid two-factor token", r.URL.Path), true)
				http.Error(w, "Invalid Two-factor token.", http.StatusUnauthorized)
				return
			}
			ttl := time.Duration(s.Broker.TwoFactorTTL) * time.Millisecond
			s.DB.Set("twofactor", id, []byte("true"), ttl)
		}

		// Check permissions exist
		if user.Permissions.Scopes == nil {
			internal.Log(ip, name, fmt.Sprintf("%s: User data is corrupted", r.URL.Path), true)
			internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
				"error": "Your user structure invalid, data corrupted.",
			})
			return
		}

		// Verify request body signature if body present
		body := internal.GetBody(r.Context())
		if body != "" {
			verified, err := brokercrypto.Verify(body, user.Key)
			if err != nil {
				internal.Log(ip, name, fmt.Sprintf("%s: Signing verification failure", r.URL.Path), true)
				internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
					"error": "Request has been tampered with!",
				})
				return
			}
			// Replace body in context with verified (unsigned) content
			ctx := internal.WithBody(r.Context(), verified)
			r = r.WithContext(ctx)
		}

		// Attach auth info to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, internal.CtxSecret, session.Secret)
		ctx = context.WithValue(ctx, internal.CtxKey, user.Key)
		ctx = context.WithValue(ctx, internal.CtxName, name)
		ctx = context.WithValue(ctx, internal.CtxUser, &user)
		ctx = context.WithValue(ctx, internal.CtxEmail, email)
		ctx = context.WithValue(ctx, internal.CtxIP, ip)

		r = r.WithContext(ctx)
		next(w, r)
	}
}
