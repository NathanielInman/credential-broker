//go:build linux || darwin

package cmd

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/NathanielInman/credential-broker/internal/keyring"
	"github.com/NathanielInman/credential-broker/internal/paths"
)

// grantdIdle is how long the grant agent stays up holding no grants.
const grantdIdle = 30 * time.Second

func newGrantdCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "grantd",
		Short:  "Hold grants in memory where there is no kernel keyring (started by broker)",
		Hidden: true,
		Args:   usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			// It holds granted values: nothing else may read its memory.
			if err := harden(); err != nil {
				return err
			}
			p, err := paths.Resolve()
			if err != nil {
				return err
			}
			if err := paths.MkdirAll(p.Run); err != nil {
				return err
			}
			// One agent per user. An agent that is just exiting holds the
			// lock for a moment, so wait briefly before deciding another
			// one is serving.
			release, err := lockFile(p.GrantLock(), time.Now().Add(2*time.Second))
			if errors.Is(err, errLocked) {
				return nil
			}
			if err != nil {
				return err
			}
			defer release()
			sock := p.GrantSocket()
			if err := os.Remove(sock); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			old := syscall.Umask(0o077)
			ln, err := net.Listen("unix", sock)
			syscall.Umask(old)
			if err != nil {
				return err
			}
			defer os.Remove(sock)
			return keyring.Serve(ln, grantdIdle)
		},
	}
}
