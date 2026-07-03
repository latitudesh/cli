package virtualmachines

import (
	"context"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

// validActions maps the CLI action names to the SDK action enum values.
var validActions = map[string]operations.CreateVirtualMachineActionVirtualMachinesAction{
	"power_on":  operations.CreateVirtualMachineActionVirtualMachinesActionPowerOn,
	"power_off": operations.CreateVirtualMachineActionVirtualMachinesActionPowerOff,
	"reboot":    operations.CreateVirtualMachineActionVirtualMachinesActionReboot,
}

func NewActionCmd() *cobra.Command {
	op := ActionVirtualMachineOperation{}
	cmd := &cobra.Command{
		Long: "Run a power action on a Virtual Machine.\n\n" +
			"Supported actions: power_on, power_off, reboot.",
		RunE:  op.run,
		Short: "Run an action on a virtual machine",
		Example: `  lsh virtual-machines action vm_xxxxxxxx --action reboot
  lsh virtual-machines action vm_xxxxxxxx --action power_off`,
		Use:  "action <id>",
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().String("action", "", "Action to perform: power_on, power_off, reboot (required)")

	return cmd
}

type ActionVirtualMachineOperation struct{}

// buildActionRequest validates the action flag and assembles the request body.
// Split out for testing without a network call.
func buildActionRequest(cmd *cobra.Command) (operations.CreateVirtualMachineActionVirtualMachinesRequestBody, error) {
	action, _ := cmd.Flags().GetString("action")
	if action == "" {
		return operations.CreateVirtualMachineActionVirtualMachinesRequestBody{}, fmt.Errorf("--action is required (power_on, power_off, reboot)")
	}
	sdkAction, ok := validActions[action]
	if !ok {
		return operations.CreateVirtualMachineActionVirtualMachinesRequestBody{}, fmt.Errorf("invalid --action %q: must be one of power_on, power_off, reboot", action)
	}

	return operations.CreateVirtualMachineActionVirtualMachinesRequestBody{
		Type: operations.CreateVirtualMachineActionVirtualMachinesTypeVirtualMachines,
		Attributes: operations.CreateVirtualMachineActionVirtualMachinesAttributes{
			Action: sdkAction,
		},
	}, nil
}

func (o *ActionVirtualMachineOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]
	body, err := buildActionRequest(cmd)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	_, err = client.VirtualMachines.CreateVirtualMachineAction(ctx, id, body, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	action, _ := cmd.Flags().GetString("action")
	if !lsh.Debug {
		fmt.Fprintf(os.Stderr, "\nAction %q submitted for virtual machine %s.\n", action, id)
	}

	return nil
}
