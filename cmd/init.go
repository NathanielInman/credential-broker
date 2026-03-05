package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	brokercrypto "github.com/nate/credential-broker/crypto"
	"github.com/nate/credential-broker/model"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize client: generate PGP keys and register with server",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if already initialized
	if _, err := os.Stat("user.json"); err == nil {
		color.Yellow("Already initialized. user.json exists.")
		overwrite, err := prompt.Confirm("Overwrite existing configuration?")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	// Prompt for broker URL
	remoteIP, err := prompt.Text("Broker URL (e.g. http://localhost:4000)")
	if err != nil {
		return err
	}
	remoteIP = strings.TrimRight(remoteIP, "/")

	// Prompt for name
	name, err := prompt.Text("Username")
	if err != nil {
		return err
	}

	// Prompt for email
	email, err := prompt.Text("Email")
	if err != nil {
		return err
	}

	// Prompt for password protection
	usePassword, err := prompt.Confirm("Protect private key with a password?")
	if err != nil {
		return err
	}

	passphrase := email
	if usePassword {
		passphrase, err = prompt.Password("Password")
		if err != nil {
			return err
		}
		confirm, err := prompt.Password("Confirm password")
		if err != nil {
			return err
		}
		if passphrase != confirm {
			return fmt.Errorf("passwords do not match")
		}
	}

	// Generate PGP keys
	fmt.Println("Generating PGP keypair...")
	privateKey, publicKey, revocationCert, err := brokercrypto.GenerateKey(name, email, passphrase)
	if err != nil {
		return fmt.Errorf("failed to generate PGP keys: %w", err)
	}

	// Save keys
	if err := os.WriteFile("id_rsa", []byte(privateKey), 0600); err != nil {
		return err
	}
	if err := os.WriteFile("id_rsa.pub", []byte(publicKey), 0644); err != nil {
		return err
	}
	if err := os.WriteFile("id_rsa.revocationCertificate", []byte(revocationCert), 0600); err != nil {
		return err
	}

	// Prompt for permissions
	fmt.Println("\nSet initial permissions:")
	viewUsers, _ := prompt.Confirm("  View users?")
	editUsers, _ := prompt.Confirm("  Edit users?")
	viewScopeNames, _ := prompt.Confirm("  View scope names?")
	createScopes, _ := prompt.Confirm("  Create scopes?")

	user := &model.User{
		RemoteIP:    remoteIP,
		Name:        name,
		Email:       email,
		UsePassword: usePassword,
		Permissions: model.Permissions{
			ViewUsers:      viewUsers,
			EditUsers:      editUsers,
			ViewScopeNames: viewScopeNames,
			CreateScopes:   createScopes,
			Scopes:         []model.ScopePermission{},
		},
	}

	// Save user.json
	if err := user.SaveUserFile("."); err != nil {
		return err
	}

	// Register with server
	fmt.Println("Registering with broker server...")
	body := map[string]interface{}{
		"name":        name,
		"email":       email,
		"key":         publicKey,
		"permissions": user.Permissions,
		"usePassword": usePassword,
	}

	resp, err := doInitRequest(remoteIP, name, email, passphrase, body)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	color.Green("Registered successfully: %s", resp)
	return nil
}

func doInitRequest(remoteIP, name, email, passphrase string, body interface{}) (string, error) {
	// Establish DH session
	prime, clientPub, clientPriv, err := brokercrypto.GenerateKeyPair()
	if err != nil {
		return "", fmt.Errorf("DH key generation failed: %w", err)
	}

	id := brokercrypto.MD5(name)

	// POST /authSecure
	headers := map[string]string{
		"id":    id,
		"key":   clientPub,
		"prime": prime,
	}

	respBody, err := doHTTPPost(remoteIP+"/authSecure", "", headers)
	if err != nil {
		return "", err
	}

	var secureResp struct {
		ServerKey string `json:"serverKey"`
	}
	if err := json.Unmarshal([]byte(respBody), &secureResp); err != nil {
		return "", fmt.Errorf("failed to parse authSecure response: %w", err)
	}

	secret, err := brokercrypto.ComputeSecret(prime, secureResp.ServerKey, clientPriv)
	if err != nil {
		return "", fmt.Errorf("DH secret computation failed: %w", err)
	}

	// POST /userInitialize with encrypted headers and body
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	initHeaders := map[string]string{
		"Content-Type": "text/plain",
		"key":          id,
		"name":         brokercrypto.AESEncrypt(name, secret),
		"email":        brokercrypto.AESEncrypt(email, secret),
	}

	resp, err := doHTTPPost(remoteIP+"/userInitialize", string(bodyJSON), initHeaders)
	if err != nil {
		return "", err
	}

	// Decrypt response: AES then PGP
	aesDecrypted, err := brokercrypto.AESDecrypt(resp, secret)
	if err != nil {
		return "", fmt.Errorf("AES decryption failed: %w", err)
	}

	pgpDecrypted, err := brokercrypto.Decrypt(aesDecrypted, "id_rsa", passphrase)
	if err != nil {
		// Response might not be PGP encrypted (e.g., error messages)
		return aesDecrypted, nil
	}

	return pgpDecrypted, nil
}
