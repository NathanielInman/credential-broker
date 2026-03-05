package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var secretDeleteCmd = &cobra.Command{
	Use:   "del [scope] [key]",
	Short: "Delete a secret from a scope",
	Args:  cobra.ExactArgs(2),
	RunE:  runSecretDelete,
}

func runSecretDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := prompt.Confirm(fmt.Sprintf("Delete secret '%s' from scope '%s'?", args[1], args[0]))
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	body := map[string]string{
		"scopeName":  args[0],
		"secretName": args[1],
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("secretDelete", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
