package main

import "os"

func platformInit() {}

// protocolOut keeps the real stdout for the protocol; Go code that prints
// goes to stderr instead. WebView2 writes nothing to stdout.
func protocolOut() (*os.File, error) {
	out := os.Stdout
	os.Stdout = os.Stderr
	return out, nil
}
