//go:build agentonly

package main

import (
	"fmt"
	"os"

	"netprob/internal/config"
)

// runServer keeps the command-line interface explicit when an agent-only
// artifact is accidentally started in server mode. Excluding server.go also
// excludes SQLite and its CGO dependency from this build.
func runServer(_ *config.Config) {
	fmt.Fprintln(os.Stderr, "server mode is not included in this agent-only build")
	os.Exit(2)
}
