package cmd

import (
	virtualmachines "github.com/latitudesh/lsh/cmd/virtualmachines"
	cobra "github.com/spf13/cobra"
)

func init() {
	virtualMachinesCmd.AddCommand(virtualmachines.NewListCmd())
	virtualMachinesCmd.AddCommand(virtualmachines.NewGetCmd())
	virtualMachinesCmd.AddCommand(virtualmachines.NewCreateCmd())
	virtualMachinesCmd.AddCommand(virtualmachines.NewUpdateCmd())
	virtualMachinesCmd.AddCommand(virtualmachines.NewDeleteCmd())
	virtualMachinesCmd.AddCommand(virtualmachines.NewActionCmd())

	rootCmd.AddCommand(virtualMachinesCmd)
}

var virtualMachinesCmd = &cobra.Command{
	Use:     "virtual-machines",
	Aliases: []string{"vm", "vms"},
	Short:   "Manage virtual machines",
	Long: "Manage the team's virtual machines: list, inspect, provision, update,\n" +
		"delete, and run power actions.",
	Example: `  lsh virtual-machines list
  lsh virtual-machines create --plan vm-small --project my-project --wait
  lsh virtual-machines action vm_xxxxxxxx --action reboot`,
}
