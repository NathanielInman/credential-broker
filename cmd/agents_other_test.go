//go:build !windows

package cmd

// stopTestAgents is a no-op off Windows: unix deletes a running binary
// fine, and the agents exit on their own once idle.
func stopTestAgents() {}
