// Package fcm sends Firebase Cloud Messaging data messages to broker's
// Android app (HTTP v1 API), and reads the two files Firebase hands out:
// the service account key the sender signs with, and the app's
// google-services.json, whose public settings the app is given at
// enrollment so the APK carries no project of its own.
//
// Unlike Web Push, FCM does not encrypt the payload to the phone: Google
// can read it. The messages only ever say that a request is waiting and
// how many secrets it wants; the app fetches the rest from the phone page.
package fcm

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NathanielInman/credential-broker/internal/config"
)

// ErrGone means FCM no longer knows the app's token: the app was
// uninstalled or its data cleared.
var ErrGone = errors.New("the app's FCM token is gone")

// scope is the OAuth scope sending needs.
const scope = "https://www.googleapis.com/auth/firebase.messaging"

// Endpoint is where messages go; tests point it elsewhere.
var Endpoint = "https://fcm.googleapis.com"

// Key is a Google service account key, the JSON Firebase's console
// downloads (Project settings, Service accounts).
type Key struct {
	ProjectID string
	email     string
	tokenURI  string
	priv      *rsa.PrivateKey
}

// ParseKey reads a service account key file's contents.
func ParseKey(data []byte) (*Key, error) {
	var f struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		PrivateKey  string `json:"private_key"`
		ClientEmail string `json:"client_email"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("service account key: %w", err)
	}
	if f.Type != "service_account" || f.ProjectID == "" || f.ClientEmail == "" || f.PrivateKey == "" {
		return nil, errors.New("service account key: not a service account key file (want type service_account, project_id, client_email, private_key)")
	}
	if u, err := url.Parse(f.TokenURI); err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		return nil, fmt.Errorf("service account key: token_uri %q is not a URL", f.TokenURI)
	}
	block, _ := pem.Decode([]byte(f.PrivateKey))
	if block == nil {
		return nil, errors.New("service account key: private_key is not PEM")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("service account key: %w", err)
	}
	priv, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("service account key: private_key is not RSA")
	}
	return &Key{ProjectID: f.ProjectID, email: f.ClientEmail, tokenURI: f.TokenURI, priv: priv}, nil
}

// App reads the public settings for the Android app named pkg out of a
// google-services.json.
func App(data []byte, pkg string) (*config.FCMApp, error) {
	var f struct {
		ProjectInfo struct {
			ProjectNumber string `json:"project_number"`
			ProjectID     string `json:"project_id"`
		} `json:"project_info"`
		Client []struct {
			ClientInfo struct {
				AppID   string `json:"mobilesdk_app_id"`
				Android struct {
					Package string `json:"package_name"`
				} `json:"android_client_info"`
			} `json:"client_info"`
			APIKey []struct {
				Current string `json:"current_key"`
			} `json:"api_key"`
		} `json:"client"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("google-services.json: %w", err)
	}
	var names []string
	for _, c := range f.Client {
		names = append(names, c.ClientInfo.Android.Package)
		if c.ClientInfo.Android.Package != pkg {
			continue
		}
		app := &config.FCMApp{ProjectID: f.ProjectInfo.ProjectID, SenderID: f.ProjectInfo.ProjectNumber, AppID: c.ClientInfo.AppID}
		if len(c.APIKey) > 0 {
			app.APIKey = c.APIKey[0].Current
		}
		if app.ProjectID == "" || app.SenderID == "" || app.AppID == "" || app.APIKey == "" {
			return nil, errors.New("google-services.json: missing project_id, project_number, mobilesdk_app_id, or api_key")
		}
		return app, nil
	}
	if len(names) == 0 {
		return nil, errors.New("google-services.json: no Android app in it")
	}
	return nil, fmt.Errorf("google-services.json: no app %s in it (has %s); add an Android app with that package name in the Firebase console", pkg, strings.Join(names, ", "))
}

// Message is a data message for the app. The app shows the notification
// itself, so a message wakes it even when it is not running.
type Message struct {
	Data map[string]string
	// TTL is how long FCM keeps it for a phone that is offline.
	TTL time.Duration
	// Collapse lets a newer message replace one still waiting.
	Collapse string
}

// Send delivers m to the app holding token, signed with key.
func Send(ctx context.Context, client *http.Client, key *Key, token string, m Message) error {
	access, err := key.accessToken(ctx, client, time.Now())
	if err != nil {
		return err
	}
	android := map[string]any{
		"priority": "HIGH",
		"ttl":      strconv.FormatInt(max(int64(m.TTL/time.Second), 0), 10) + "s",
	}
	if m.Collapse != "" {
		android["collapse_key"] = m.Collapse
	}
	body, err := json.Marshal(map[string]any{"message": map[string]any{
		"token":   token,
		"data":    m.Data,
		"android": android,
	}})
	if err != nil {
		return err
	}
	u := Endpoint + "/v1/projects/" + url.PathEscape(key.ProjectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode/100 == 2 {
		return nil
	}
	// A token FCM has dropped is 404 UNREGISTERED; a malformed one is 400
	// INVALID_ARGUMENT, which also covers bad payloads, so only the first
	// counts as gone.
	if resp.StatusCode == http.StatusNotFound || bytes.Contains(msg, []byte(`"UNREGISTERED"`)) {
		return ErrGone
	}
	return fmt.Errorf("FCM: %s: %s", resp.Status, errorText(msg))
}

// errorText is the message in a Google API error body, or the body.
func errorText(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(body, &e) == nil {
		if e.Error.Message != "" {
			return e.Error.Message
		}
		if e.Description != "" {
			return e.Description
		}
	}
	return string(bytes.TrimSpace(body))
}

var b64 = base64.RawURLEncoding

// Check signs in with key, which proves Google takes it.
func (k *Key) Check(ctx context.Context, client *http.Client) error {
	_, err := k.accessToken(ctx, client, time.Now())
	return err
}

// accessToken trades a signed JWT for an OAuth access token (RFC 7523).
// Each approver is a new process sending one message, so nothing is
// cached.
func (k *Key) accessToken(ctx context.Context, client *http.Client, now time.Time) (string, error) {
	jwt, err := k.sign(now)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {jwt}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("FCM sign-in: %s: %s", resp.Status, errorText(body))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", errors.New("FCM sign-in: no access token in the answer")
	}
	return tok.AccessToken, nil
}

// sign makes the JWT the token endpoint takes, valid for an hour (the
// most Google accepts).
func (k *Key) sign(now time.Time) (string, error) {
	header := b64.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{
		"iss":   k.email,
		"scope": scope,
		"aud":   k.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	unsigned := header + "." + b64.EncodeToString(claims)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.priv, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + b64.EncodeToString(sig), nil
}
