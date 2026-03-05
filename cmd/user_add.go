package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/model"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var userAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserAdd,
}

func runUserAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	email, err := prompt.Text("Email")
	if err != nil {
		return err
	}

	// Read the new user's public key
	keyPath, err := prompt.Text("Path to user's public key file")
	if err != nil {
		return err
	}
	keyBytes, err := readFileBytes(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	fmt.Println("Set permissions:")
	viewUsers, _ := prompt.Confirm("  View users?")
	editUsers, _ := prompt.Confirm("  Edit users?")
	viewScopeNames, _ := prompt.Confirm("  View scope names?")
	createScopes, _ := prompt.Confirm("  Create scopes?")

	body := map[string]interface{}{
		"name":  name,
		"email": email,
		"key":   string(keyBytes),
		"permissions": model.Permissions{
			ViewUsers:      viewUsers,
			EditUsers:      editUsers,
			ViewScopeNames: viewScopeNames,
			CreateScopes:   createScopes,
			Scopes:         []model.ScopePermission{},
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("userAdd", string(bodyJSON))
	if err != nil {
		return err
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		color.Green("%s", resp)
		return nil
	}

	if msg, ok := result["success"]; ok {
		color.Green("%s", msg)
	} else if msg, ok := result["error"]; ok {
		color.Red("%s", msg)
	}

	return nil
}

func readFileBytes(path string) ([]byte, error) {
	return readFileContent(path)
}
