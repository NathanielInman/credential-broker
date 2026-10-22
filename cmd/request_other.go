//go:build !linux && !darwin

package cmd

import (
	"syscall"

	"github.com/spf13/cobra"
	"time"

	"github.com/NathanielInman/credential-broker/internal/paths"
)

func newSession() *syscall.SysProcAttr { return nil }

func lockRequests(paths.Paths, time.Time) (func(), error) { return nil, errUnsupported }

func lockFile(string, time.Time) (func(), error) { return nil, errUnsupported }

func newGrantdCmd() *cobra.Command {
	return &cobra.Command{Use: "grantd", Hidden: true, RunE: func(*cobra.Command, []string) error { return errUnsupported }}
}
