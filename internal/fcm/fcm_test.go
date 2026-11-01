package fcm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// keyFile makes a service account key file whose token_uri is tokenURI.
func keyFile(t *testing.T, tokenURI string) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "broker-1",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email": "sender@broker-1.iam.gserviceaccount.com",
		"token_uri":    tokenURI,
	})
	return data, priv
}

func TestParseKey(t *testing.T) {
	data, _ := keyFile(t, "https://oauth2.googleapis.com/token")
	k, err := ParseKey(data)
	if err != nil || k.ProjectID != "broker-1" {
		t.Fatalf("ParseKey: %v %+v", err, k)
	}
	for name, bad := range map[string]string{
		"not json":  "{",
		"no key":    `{"type":"service_account","project_id":"p","client_email":"e","token_uri":"https://x"}`,
		"user cred": `{"type":"authorized_user"}`,
	} {
		if _, err := ParseKey([]byte(bad)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

const googleServices = `{
  "project_info": {"project_number": "123", "project_id": "broker-1"},
  "client": [
    {"client_info": {"mobilesdk_app_id": "1:123:android:other", "android_client_info": {"package_name": "com.example.other"}},
     "api_key": [{"current_key": "AIza-other"}]},
    {"client_info": {"mobilesdk_app_id": "1:123:android:abc", "android_client_info": {"package_name": "com.nathanielinman.broker"}},
     "api_key": [{"current_key": "AIza-test"}]}
  ]
}`

func TestApp(t *testing.T) {
	app, err := App([]byte(googleServices), "com.nathanielinman.broker")
	if err != nil {
		t.Fatal(err)
	}
	if app.ProjectID != "broker-1" || app.SenderID != "123" || app.AppID != "1:123:android:abc" || app.APIKey != "AIza-test" {
		t.Fatalf("App: %+v", app)
	}
	if _, err := App([]byte(googleServices), "com.example.missing"); err == nil || !strings.Contains(err.Error(), "com.example.other") {
		t.Fatalf("missing package: %v", err)
	}
}

// TestSend signs in with a JWT the fake token endpoint verifies, then
// sends a high-priority data message with the access token it got.
func TestSend(t *testing.T) {
	var priv *rsa.PrivateKey
	var sent map[string]any
	gone := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			parts := strings.Split(r.PostForm.Get("assertion"), ".")
			if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || len(parts) != 3 {
				http.Error(w, `{"error_description":"bad grant"}`, 400)
				return
			}
			sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			sig, _ := b64.DecodeString(parts[2])
			if rsa.VerifyPKCS1v15(&priv.PublicKey, crypto.SHA256, sum[:], sig) != nil {
				http.Error(w, `{"error_description":"bad signature"}`, 400)
				return
			}
			claims, _ := b64.DecodeString(parts[1])
			var c map[string]any
			_ = json.Unmarshal(claims, &c)
			if c["scope"] != scope || c["iss"] != "sender@broker-1.iam.gserviceaccount.com" || !strings.HasSuffix(c["aud"].(string), "/token") {
				http.Error(w, `{"error_description":"bad claims"}`, 400)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"ya29.test","expires_in":3599}`)
		case "/v1/projects/broker-1/messages:send":
			if r.Header.Get("Authorization") != "Bearer ya29.test" {
				http.Error(w, "unauthorized", 401)
				return
			}
			if gone {
				w.WriteHeader(404)
				_, _ = io.WriteString(w, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&sent)
			_, _ = io.WriteString(w, `{"name":"projects/broker-1/messages/1"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := Endpoint
	Endpoint = srv.URL
	defer func() { Endpoint = old }()

	data, p := keyFile(t, srv.URL+"/token")
	priv = p
	key, err := ParseKey(data)
	if err != nil {
		t.Fatal(err)
	}
	m := Message{Data: map[string]string{"type": "request", "count": "2"}, TTL: 90 * time.Second, Collapse: "request"}
	if err := Send(context.Background(), srv.Client(), key, "device-token", m); err != nil {
		t.Fatal(err)
	}
	msg := sent["message"].(map[string]any)
	android := msg["android"].(map[string]any)
	if msg["token"] != "device-token" || msg["data"].(map[string]any)["count"] != "2" ||
		android["priority"] != "HIGH" || android["ttl"] != "90s" || android["collapse_key"] != "request" {
		t.Fatalf("sent %v", sent)
	}
	if _, ok := msg["notification"]; ok {
		t.Fatal("a notification message would skip the app")
	}

	gone = true
	if err := Send(context.Background(), srv.Client(), key, "device-token", m); !errors.Is(err, ErrGone) {
		t.Fatalf("unregistered token: %v", err)
	}
}
