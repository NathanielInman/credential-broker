package cmd

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/fcm"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// fcmServer is Google's token endpoint and FCM in one, handing out any
// access token and keeping each message it gets.
func fcmServer(t *testing.T) (*httptest.Server, chan map[string]any) {
	got := make(chan map[string]any, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			io.WriteString(w, `{"access_token":"ya29.test"}`)
		case strings.HasSuffix(r.URL.Path, "/messages:send"):
			var m map[string]any
			json.NewDecoder(r.Body).Decode(&m)
			got <- m["message"].(map[string]any)
			io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := fcm.Endpoint
	fcm.Endpoint = srv.URL
	t.Cleanup(func() { fcm.Endpoint = old })
	return srv, got
}

func TestPhoneFCM(t *testing.T) {
	grantVault(t)
	requireTTY = func(io.Reader, string) error { return nil }
	t.Cleanup(func() { requireTTY = defaultRequireTTY })
	srv, got := fcmServer(t)

	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	key, _ := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "broker-1", "client_email": "s@broker-1.iam.gserviceaccount.com",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), "token_uri": srv.URL + "/token",
	})
	dir := t.TempDir()
	keyFile, gsFile := filepath.Join(dir, "key.json"), filepath.Join(dir, "google-services.json")
	os.WriteFile(keyFile, key, 0o600)
	os.WriteFile(gsFile, []byte(`{"project_info":{"project_number":"123","project_id":"broker-1"},"client":[{"client_info":
		{"mobilesdk_app_id":"1:123:android:abc","android_client_info":{"package_name":"`+androidPackage+`"}},"api_key":[{"current_key":"AIza-test"}]}]}`), 0o600)

	if out, _ := mustRun(t, "phone", "fcm", keyFile, gsFile); !strings.Contains(out, "Firebase project broker-1") {
		t.Fatalf("phone fcm: %q", out)
	}
	p, _ := paths.Resolve()
	if data, err := os.ReadFile(p.FCMKey()); err != nil || !bytes.Equal(data, key) {
		t.Fatalf("saved key: %v", err)
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil || cfg.FCM == nil || cfg.FCM.AppID != "1:123:android:abc" || cfg.FCM.APIKey != "AIza-test" {
		t.Fatalf("config FCM: %v %+v", err, cfg.FCM)
	}

	// An app phone gets its request through FCM, saying only how many.
	cfg.Phone = &config.PhonePage{Origin: "https://broker.test"}
	cfg.Unlock = append(cfg.Unlock, config.Unlock{Phone: &config.PhoneKey{Name: "moto", App: true, FCM: "device-token"}})
	var stderr bytes.Buffer
	notifyPhones(p, cfg, 2, time.Now().Add(time.Minute), &stderr)
	m := <-got
	data := m["data"].(map[string]any)
	if m["token"] != "device-token" || data["type"] != "request" || data["count"] != "2" || stderr.Len() != 0 {
		t.Fatalf("message %v, stderr %q", m, stderr.String())
	}

	// Files that do not belong together are refused.
	os.WriteFile(gsFile, bytes.ReplaceAll(must(os.ReadFile(gsFile)), []byte("broker-1"), []byte("other")), 0o600)
	if _, stderr, code := runStdin(t, "", "phone", "fcm", keyFile, gsFile); code != 64 || !strings.Contains(stderr, "project") {
		t.Fatalf("mismatched files: %d %s", code, stderr)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
