package cmd

import (
	traffic "github.com/latitudesh/lsh/cmd/traffic"
	cobra "github.com/spf13/cobra"
)

func init() {
	trafficCmd.AddCommand(traffic.NewListCmd())
	trafficCmd.AddCommand(traffic.NewQuotaCmd())

	rootCmd.AddCommand(trafficCmd)
}

var trafficCmd = &cobra.Command{Use: "traffic", Short: "Inspect traffic consumption and quota"}
