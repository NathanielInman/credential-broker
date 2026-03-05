package cmd

import (
	"fmt"

	"github.com/nate/credential-broker/server"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the broker server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Credential Broker server...")
		return server.Start()
	},
}
