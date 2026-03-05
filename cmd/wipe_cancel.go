package cmd

import (
	"fmt"

	"github.com/nate/credential-broker/client"
	"github.com/spf13/cobra"
)

var wipeCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a pending wipe",
	RunE:  runWipeCancel,
}

func runWipeCancel(cmd *cobra.Command, args []string) error {
	resp, err := client.Request("wipeCancel", "")
	if err != nil {
		return fmt.Errorf("wipe cancel failed: %w", err)
	}

	printResponse(resp)
	return nil
}
