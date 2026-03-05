package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

var userDeleteCmd = &cobra.Command{
	Use:   "del [name]",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserDelete,
}

func runUserDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	confirmed, err := prompt.Confirm(fmt.Sprintf("Delete user '%s'?", name))
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	body := map[string]string{"target": name}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("userDelete", string(bodyJSON))
	if err != nil {
		return err
	}

	printResponse(resp)
	return nil
}

func printResponse(resp string) {
	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Println(resp)
		return
	}
	if msg, ok := result["success"]; ok {
		color.Green("%s", msg)
	} else if msg, ok := result["error"]; ok {
		color.Red("%s", msg)
	}
}
