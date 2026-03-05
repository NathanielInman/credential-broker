package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/nate/credential-broker/client"
	"github.com/spf13/cobra"
)

var scopeGetCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Get scope(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runScopeGet,
}

// getCmd is a top-level shortcut: `broker get [scope]`
var getCmd = &cobra.Command{
	Use:   "get [scope]",
	Short: "Get secrets for a scope (shortcut for 'scope get')",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runScopeGet,
}

func runScopeGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Get all scope names
		resp, err := client.Request("scopeGetAll", "")
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

		fmt.Println("Scopes:")
		for _, name := range result.Success {
			color.Cyan("  %s", name)
		}
		return nil
	}

	// Get specific scope
	body := map[string]string{"scopeName": args[0]}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := client.Request("scopeGet", string(bodyJSON))
	if err != nil {
		return err
	}

	var result struct {
		Success map[string]string `json:"success"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		fmt.Println(resp)
		return nil
	}

	if result.Error != "" {
		color.Red("%s", result.Error)
		return nil
	}

	if len(result.Success) == 0 {
		fmt.Println("No secrets in this scope.")
		return nil
	}

	for key, value := range result.Success {
		fmt.Printf("  %s: %s\n", color.CyanString(key), value)
	}
	return nil
}
