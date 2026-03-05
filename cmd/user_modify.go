package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/model"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var userModifyCmd = &cobra.Command{
	Use:   "mod [name]",
	Short: "Modify a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserModify,
}

func runUserModify(cmd *cobra.Command, args []string) error {
	name := args[0]

	fmt.Printf("Modifying user '%s'. Leave blank to keep current value.\n", name)

	email, err := prompt.Text("New email (blank to skip)")
	if err != nil {
		return err
	}

	fmt.Println("Set permissions:")
	viewUsers, _ := prompt.Confirm("  View users?")
	editUsers, _ := prompt.Confirm("  Edit users?")
	viewScopeNames, _ := prompt.Confirm("  View scope names?")
	createScopes, _ := prompt.Confirm("  Create scopes?")

	body := map[string]interface{}{
		"name": name,
		"permissions": model.Permissions{
			ViewUsers:      viewUsers,
			EditUsers:      editUsers,
			ViewScopeNames: viewScopeNames,
			CreateScopes:   createScopes,
			Scopes:         []model.ScopePermission{},
		},
	}

	if email != "" {
		body["email"] = email
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("userModify", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}
