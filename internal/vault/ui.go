package vault

import (
	"fmt"
	"io"

	"filippo.io/age/plugin"
)

// TerminalUI returns the ClientUI for plugin prompts ("touch your YubiKey").
// Messages go to w, normally stderr; input is read from /dev/tty, so a
// redirected stdin or stdout does not swallow the prompt.
func TerminalUI(w io.Writer) *plugin.ClientUI {
	return plugin.NewTerminalUI(
		func(format string, v ...any) { fmt.Fprintf(w, "broker: "+format+"\n", v...) },
		func(format string, v ...any) { fmt.Fprintf(w, "broker: warning: "+format+"\n", v...) },
	)
}
