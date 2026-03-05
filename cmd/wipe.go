package cmd

import (
	"fmt"

	"github.com/nate/credential-broker/client"
	"github.com/nate/credential-broker/prompt"
	"github.com/spf13/cobra"
)

func runWipe(cmd *cobra.Command, args []string) error {
	confirmed, err := prompt.Confirm("This will wipe ALL users from the broker. Are you sure?")
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	confirmed2, err := prompt.Confirm("Are you absolutely certain? This cannot be undone.")
	if err != nil {
		return err
	}
	if !confirmed2 {
		return nil
	}

	resp, err := client.Request("wipe", "")
	if err != nil {
		return fmt.Errorf("wipe failed: %w", err)
	}

	printResponse(resp)
	return nil
}
