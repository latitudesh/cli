package main

import (
	"os"

	"github.com/latitudesh/lsh/cmd"
)

func main() {
	// Propagate command failures as a non-zero exit code. Cobra already prints
	// the error (commands set SilenceUsage), so we only need to set the status
	// — scripts, CI and AI agents rely on it.
	if _, err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
