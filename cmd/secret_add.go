package cmd

import (
	"encoding/json"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var secretAddCmd = &cobra.Command{
	Use:   "add [scope] [key]",
	Short: "Add a secret to a scope",
	Args:  cobra.ExactArgs(2),
	RunE:  runSecretAdd,
}

func runSecretAdd(cmd *cobra.Command, args []string) error {
	scopeName := args[0]
	secretName := args[1]

	secretValue, err := prompt.Password("Secret value")
	if err != nil {
		return err
	}

	body := map[string]string{
		"scopeName":   scopeName,
		"secretName":  secretName,
		"secretValue": secretValue,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("secretAdd", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
