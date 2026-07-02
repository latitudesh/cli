package cmd

import (
	firewalls "github.com/latitudesh/lsh/cmd/firewalls"
	cobra "github.com/spf13/cobra"
)

func init() {
	firewallsCmd.AddCommand(firewalls.NewListCmd())
	firewallsCmd.AddCommand(firewalls.NewGetCmd())
	firewallsCmd.AddCommand(firewalls.NewCreateCmd())
	firewallsCmd.AddCommand(firewalls.NewUpdateCmd())
	firewallsCmd.AddCommand(firewalls.NewDeleteCmd())
	firewallsCmd.AddCommand(firewalls.NewAssignmentsCmd())

	rootCmd.AddCommand(firewallsCmd)
}

var firewallsCmd = &cobra.Command{
	Use:   "firewalls",
	Short: "Manage firewalls",
	Long: "Manage the team's firewalls and their server assignments.\n\n" +
		"Firewalls hold a set of rules that can be attached to servers through\n" +
		"the assignments subcommand.",
	Example: `  lsh firewalls create --name web --project my-project --rules @rules.json
  lsh firewalls list
  lsh firewalls assignments create --firewall fw_xxxxxxxx --server sv_xxxxxxxx`,
}
