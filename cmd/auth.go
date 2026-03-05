package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/mdp/qrterminal/v3"
	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
	"os"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with 2FA setup",
	RunE:  runAuth,
}

func runAuth(cmd *cobra.Command, args []string) error {
	// Request 2FA initialization
	resp, err := client.Request("auth2FAInitialize", "")
	if err != nil {
		return fmt.Errorf("2FA initialization failed: %w", err)
	}

	var initResp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(resp), &initResp); err != nil {
		return fmt.Errorf("failed to parse 2FA response: %w", err)
	}

	// Display QR code
	fmt.Println("\nScan this QR code with your authenticator app:")
	qrterminal.GenerateWithConfig(initResp.URL, qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    os.Stdout,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
	})

	// Prompt for token
	token, err := prompt.Text("Enter 6-digit token")
	if err != nil {
		return err
	}

	// Validate token
	validateResp, err := client.Request("auth2FAValidate", token)
	if err != nil {
		return fmt.Errorf("2FA validation failed: %w", err)
	}

	var validateResult struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(validateResp), &validateResult); err != nil {
		return fmt.Errorf("failed to parse validation response: %w", err)
	}

	if validateResult.Valid {
		color.Green("Two-factor authentication enabled successfully.")
	} else {
		color.Red("Invalid token. Please try again.")
	}

	return nil
}
