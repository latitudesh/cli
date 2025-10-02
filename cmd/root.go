package cmd

import (
	"log"
	"os"
	"path/filepath"

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

// Execute executes the root command.
func Execute() (*cobra.Command, error) {
	cmd, err := cli.MakeRootCmd(rootCmd)
	if err != nil {
		log.Fatal("Cmd construction error: ", err)
	}

	rootCmd.PersistentFlags().BoolVar(&lsh.DryRun, "dry-run", false, "do not send the request to server")
	rootCmd.PersistentFlags().BoolVar(&lsh.Debug, "debug", false, "output debug logs")

	return cmd, cmd.Execute()
}
