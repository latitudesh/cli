package cmd

import (
	events "github.com/latitudesh/lsh/cmd/events"
	cobra "github.com/spf13/cobra"
)

func init() {
	eventsCmd.AddCommand(events.NewListCmd())

	rootCmd.AddCommand(eventsCmd)
}

var eventsCmd = &cobra.Command{Use: "events", Short: "Audit team events"}
