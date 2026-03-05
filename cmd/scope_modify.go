package cmd

import (
	"encoding/json"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var scopeModifyCmd = &cobra.Command{
	Use:   "mod [name]",
	Short: "Modify a scope (rename)",
	Args:  cobra.ExactArgs(1),
	RunE:  runScopeModify,
}

func runScopeModify(cmd *cobra.Command, args []string) error {
	newName, err := prompt.Text("New scope name")
	if err != nil {
		return err
	}

	body := map[string]string{
		"target":    args[0],
		"scopeName": newName,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("scopeModify", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
