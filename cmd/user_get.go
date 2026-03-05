package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/nate/credential-broker/client"
	"github.com/spf13/cobra"
)

var userGetCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Get user(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUserGet,
}

func runUserGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Get all users
		resp, err := client.Request("userGetAll", "")
		if err != nil {
			return err
		}

		var result struct {
			Success []string `json:"success"`
		}
		if err := json.Unmarshal([]byte(resp), &result); err != nil {
			fmt.Println(resp)
			return nil
		}

		fmt.Println("Users:")
		for _, name := range result.Success {
			color.Cyan("  %s", name)
		}
		return nil
	}

	// Get specific user
	body := map[string]string{"name": args[0]}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("userGet", string(bodyJSON))
	if err != nil {
		return err
	}

	var result struct {
		Success json.RawMessage `json:"success"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Println(resp)
		return nil
	}

	if result.Error != "" {
		color.Red("%s", result.Error)
		return nil
	}

	formatted, _ := json.MarshalIndent(json.RawMessage(result.Success), "", "  ")
	fmt.Println(string(formatted))
	return nil
}
