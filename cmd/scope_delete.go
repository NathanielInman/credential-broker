package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var scopeDeleteCmd = &cobra.Command{
	Use:   "del [name]",
	Short: "Delete a scope",
	Args:  cobra.ExactArgs(1),
	RunE:  runScopeDelete,
}

func runScopeDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := prompt.Confirm(fmt.Sprintf("Delete scope '%s'?", args[0]))
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	body := map[string]string{"scopeName": args[0]}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("scopeDelete", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
