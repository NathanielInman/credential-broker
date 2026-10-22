package main

import (
	"os"

	"github.com/NathanielInman/credential-broker/cmd"
)

// version is set by the release build.
var version = "dev"

func main() {
	os.Exit(cmd.Execute(version))
}
