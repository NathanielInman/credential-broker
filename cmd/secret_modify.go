package cmd

import (
	"encoding/json"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var secretModifyCmd = &cobra.Command{
	Use:   "mod [scope] [key]",
	Short: "Modify a secret in a scope",
	Args:  cobra.ExactArgs(2),
	RunE:  runSecretModify,
}

func runSecretModify(cmd *cobra.Command, args []string) error {
	secretValue, err := prompt.Password("New secret value")
	if err != nil {
		return err
	}

	body := map[string]string{
		"scopeName":   args[0],
		"secretName":  args[1],
		"secretValue": secretValue,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("secretModify", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
