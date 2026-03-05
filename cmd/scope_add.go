package cmd

import (
	"encoding/json"

	"github.com/nate/credential-broker/client"
	"github.com/spf13/cobra"
)

var scopeAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new scope",
	Args:  cobra.ExactArgs(1),
	RunE:  runScopeAdd,
}

func runScopeAdd(cmd *cobra.Command, args []string) error {
	body := map[string]string{"scopeName": args[0]}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("scopeAdd", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
