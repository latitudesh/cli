package main

import (
	"os"

	"github.com/latitudesh/lsh/cmd"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func main() {
	// Propagate command failures as a non-zero exit code. Cobra already prints
	// the error (commands set SilenceUsage), so we only need to set the status
	// — scripts, CI and AI agents rely on it. Commands that attach an explicit
	// exit code (see internal/exitcode) get it; everything else exits with 1.
	if _, err := cmd.Execute(); err != nil {
		os.Exit(exitcode.Of(err))
	}
}
