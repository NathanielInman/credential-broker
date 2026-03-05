package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "broker",
	Short: "Secure, distributed secret management",
	Long:  "Credential Broker - secure, distributed secret management for DevOps credential management.",
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

var scopeCmd = &cobra.Command{
	Use:   "scope",
	Short: "Manage scopes",
}

var wipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe all users from the broker",
	RunE:  runWipe,
}

func init() {
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(scopeCmd)
	rootCmd.AddCommand(wipeCmd)

	// User subcommands
	userCmd.AddCommand(userAddCmd)
	userCmd.AddCommand(userDeleteCmd)
	userCmd.AddCommand(userGetCmd)
	userCmd.AddCommand(userModifyCmd)

	// Scope subcommands
	scopeCmd.AddCommand(scopeAddCmd)
	scopeCmd.AddCommand(scopeDeleteCmd)
	scopeCmd.AddCommand(scopeGetCmd)
	scopeCmd.AddCommand(scopeModifyCmd)

	// Wipe subcommands
	wipeCmd.AddCommand(wipeCancelCmd)

	// Top-level commands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(secretAddCmd)
	rootCmd.AddCommand(secretDeleteCmd)
	rootCmd.AddCommand(secretModifyCmd)
	rootCmd.AddCommand(getCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
