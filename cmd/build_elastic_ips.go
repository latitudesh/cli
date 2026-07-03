package cmd

import (
	elasticips "github.com/latitudesh/lsh/cmd/elasticips"
	cobra "github.com/spf13/cobra"
)

func init() {
	elasticIPsCmd.AddCommand(elasticips.NewListCmd())
	elasticIPsCmd.AddCommand(elasticips.NewGetCmd())
	elasticIPsCmd.AddCommand(elasticips.NewCreateCmd())
	elasticIPsCmd.AddCommand(elasticips.NewUpdateCmd())
	elasticIPsCmd.AddCommand(elasticips.NewDeleteCmd())

	rootCmd.AddCommand(elasticIPsCmd)
}

var elasticIPsCmd = &cobra.Command{
	Use:   "elastic-ips",
	Short: "Manage Elastic IPs",
	Long: "Manage the team's Elastic IPs.\n\n" +
		"Elastic IPs are IPv4 addresses that can be allocated to a server and moved\n" +
		"between servers. Allocation is asynchronous; a new IP starts in the\n" +
		"'configuring' status.",
	Example: `  lsh elastic-ips create --project my-project --server sv_xxxxxxxx
  lsh elastic-ips list
  lsh elastic-ips delete eip_xxxxxxxx`,
}
