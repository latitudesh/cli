package cmd

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/spf13/cobra"
)

var (
	exeName string = filepath.Base(os.Args[0])

	// Use executable name as the command name
	rootCmd = &cobra.Command{
		Use:     exeName,
		Version: version.Version,
	}
)

var (
	rootOnce sync.Once
	rootErr  error
)

// Root builds the complete command tree without running anything. Execute
// uses it, and so does anything that needs the tree without executing a
// command, such as tests and tooling. The tree hangs off the package-level
// rootCmd and registers flags on it, so it is built once: a second
// MakeRootCmd would panic on the redefined flags.
func Root() (*cobra.Command, error) {
	rootOnce.Do(func() {
		if _, rootErr = cli.MakeRootCmd(rootCmd); rootErr != nil {
			return
		}

		rootCmd.PersistentFlags().BoolVar(&lsh.DryRun, "dry-run", false, "log the request that would be sent and exit without calling the API")
		rootCmd.PersistentFlags().BoolVar(&lsh.Debug, "debug", false, "print debug logs to stderr")

		// Attach the SDK-based project-scoped groups (`projects ssh-keys`,
		// `projects user-data`) to the generated `projects` command, which
		// only exists after MakeRootCmd has run.
		wireProjectScopedCommands(rootCmd)
	})
	if rootErr != nil {
		return nil, rootErr
	}
	return rootCmd, nil
}

// Execute executes the root command.
func Execute() (*cobra.Command, error) {
	cmd, err := Root()
	if err != nil {
		log.Fatal("Cmd construction error: ", err)
	}
	return cmd, cmd.Execute()
}
