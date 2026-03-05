package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	brokercrypto "github.com/nate/credential-broker/crypto"
	"github.com/nate/credential-broker/model"
	"github.com/nate/credential-broker/prompt"
)

func loadUser() (*model.User, error) {
	if _, err := os.Stat("user.json"); os.IsNotExist(err) {
		return nil, fmt.Errorf("no user exists locally, initialize first with \"broker init\"")
	}
	return model.LoadUserFile(".")
}

func getPassphrase(user *model.User) (string, error) {
	if user.UsePassword {
		return prompt.Password("Private key password")
	}
	return user.Email, nil
}

func httpPost(url, body string, headers map[string]string) (string, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

func authSecure(user *model.User) error {
	prime, clientPub, clientPriv, err := brokercrypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("DH key generation failed: %w", err)
	}

	id := brokercrypto.MD5(user.Name)

	headers := map[string]string{
		"id":    id,
		"key":   clientPub,
		"prime": prime,
	}

	respBody, err := httpPost(user.RemoteIP+"/authSecure", "", headers)
	if err != nil {
		return fmt.Errorf("authSecure request failed: %w", err)
	}

	// The server returns the server key directly as JSON (could be quoted string or object)
	var serverKey string
	if err := json.Unmarshal([]byte(respBody), &serverKey); err != nil {
		// Try as object with serverKey field
		var resp struct {
			ServerKey string `json:"serverKey"`
		}
		if err2 := json.Unmarshal([]byte(respBody), &resp); err2 != nil {
			return fmt.Errorf("failed to parse authSecure response: %s", respBody)
		}
		serverKey = resp.ServerKey
	}

	secret, err := brokercrypto.ComputeSecret(prime, serverKey, clientPriv)
	if err != nil {
		return fmt.Errorf("DH secret computation failed: %w", err)
	}

	user.Secret = secret
	return user.SaveUserFile(".")
}

func authenticate(user *model.User) error {
	if err := authSecure(user); err != nil {
		return err
	}

	id := brokercrypto.MD5(user.Name)
	passphrase, err := getPassphrase(user)
	if err != nil {
		return err
	}

	// Step 1: authIdentify
	headers := map[string]string{
		"Content-Type": "text/plain",
		"id":           id,
		"name":         brokercrypto.AESEncrypt(user.Name, user.Secret),
		"email":        brokercrypto.AESEncrypt(user.Email, user.Secret),
	}

	identifyResp, err := httpPost(user.RemoteIP+"/authIdentify", "", headers)
	if err != nil {
		return fmt.Errorf("authIdentify failed: %w", err)
	}

	// Decrypt: AES then PGP
	aesDecrypted, err := brokercrypto.AESDecrypt(identifyResp, user.Secret)
	if err != nil {
		return fmt.Errorf("AES decrypt of challenge failed: %w", err)
	}

	privateKey, err := os.ReadFile("id_rsa")
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	challenge, err := brokercrypto.Decrypt(aesDecrypted, string(privateKey), passphrase)
	if err != nil {
		return fmt.Errorf("PGP decrypt of challenge failed: %w", err)
	}

	// Parse the challenge (it's JSON-encoded)
	var challengeStr string
	if err := json.Unmarshal([]byte(challenge), &challengeStr); err != nil {
		challengeStr = challenge
	}

	// Step 2: authChallenge - send MD5 of challenge
	challengeHash := brokercrypto.MD5(challengeStr)

	challengeHeaders := map[string]string{
		"Content-Type": "text/plain",
		"id":           id,
		"name":         brokercrypto.AESEncrypt(user.Name, user.Secret),
		"email":        brokercrypto.AESEncrypt(user.Email, user.Secret),
		"challenge":    brokercrypto.AESEncrypt(challengeHash, user.Secret),
	}

	challengeResp, err := httpPost(user.RemoteIP+"/authChallenge", "", challengeHeaders)
	if err != nil {
		return fmt.Errorf("authChallenge failed: %w", err)
	}

	// Decrypt response
	aesDecrypted, err = brokercrypto.AESDecrypt(challengeResp, user.Secret)
	if err != nil {
		return fmt.Errorf("AES decrypt of auth response failed: %w", err)
	}

	_, err = brokercrypto.Decrypt(aesDecrypted, string(privateKey), passphrase)
	if err != nil {
		// might not be PGP encrypted, that's ok
	}

	return nil
}

func makeRequest(user *model.User, uri, body, twoFactorToken string) (string, error) {
	id := brokercrypto.MD5(user.Name)

	headers := map[string]string{
		"Content-Type": "text/plain",
		"id":           id,
		"name":         brokercrypto.AESEncrypt(user.Name, user.Secret),
		"email":        brokercrypto.AESEncrypt(user.Email, user.Secret),
	}

	if twoFactorToken != "" {
		headers["Two-Factor-Token"] = twoFactorToken
	}

	return httpPost(user.RemoteIP+"/"+uri, body, headers)
}

// Request performs a full authenticated request to the broker server.
// It handles session establishment, identity challenge, 2FA, and request signing.
func Request(uri, body string) (string, error) {
	user, err := loadUser()
	if err != nil {
		return "", err
	}

	fmt.Print(color.YellowString("Synchronizing with server... "))

	passphrase, err := getPassphrase(user)
	if err != nil {
		return "", err
	}

	// Sign body if not empty
	signedBody := body
	if body != "" {
		privateKey, err := os.ReadFile("id_rsa")
		if err != nil {
			return "", fmt.Errorf("failed to read private key: %w", err)
		}
		signedBody, err = brokercrypto.Sign(body, string(privateKey), passphrase)
		if err != nil {
			return "", fmt.Errorf("failed to sign request body: %w", err)
		}
	}

	// Ensure we have a session
	if user.Secret == "" {
		if err := authSecure(user); err != nil {
			return "", err
		}
	}

	// Make request
	resp, err := makeRequest(user, uri, signedBody, "")
	if err != nil {
		return "", err
	}

	// Handle session expiry
	if resp == "Session expired." {
		if err := authenticate(user); err != nil {
			return "", fmt.Errorf("re-authentication failed: %w", err)
		}
		resp, err = makeRequest(user, uri, signedBody, "")
		if err != nil {
			return "", err
		}
	}

	// Handle 2FA expiry
	if resp == "Two-factor expired." {
		fmt.Println("(2FA required)")

		var twoFactorToken string
		for {
			twoFactorToken, err = prompt.Text(color.GreenString("Enter two-factor token: "))
			if err != nil {
				return "", err
			}
			confirmed, err := prompt.Confirm(fmt.Sprintf("Is this correct: \"%s\"?", twoFactorToken))
			if err != nil {
				return "", err
			}
			if confirmed {
				break
			}
			fmt.Println(color.GreenString("No problem, let's try again."))
		}

		resp, err = makeRequest(user, uri, signedBody, twoFactorToken)
		if err != nil {
			return "", err
		}
	}

	// Decrypt response: AES then PGP
	aesDecrypted, err := brokercrypto.AESDecrypt(resp, user.Secret)
	if err != nil {
		// Response might not be encrypted (error messages)
		fmt.Println(color.GreenString("(done)"))
		return resp, nil
	}

	privateKey, err := os.ReadFile("id_rsa")
	if err != nil {
		fmt.Println(color.GreenString("(done)"))
		return aesDecrypted, nil
	}

	pgpDecrypted, err := brokercrypto.Decrypt(aesDecrypted, string(privateKey), passphrase)
	if err != nil {
		fmt.Println(color.GreenString("(done)"))
		return aesDecrypted, nil
	}

	fmt.Println(color.GreenString("(done)"))
	return pgpDecrypted, nil
}
