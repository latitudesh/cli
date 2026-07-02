package virtualmachines

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetVirtualMachineOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single Virtual Machine by its ID.\n",
		RunE:    op.run,
		Short:   "Get a virtual machine",
		Example: `  lsh virtual-machines get vm_xxxxxxxx`,
		Use:     "get <id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type GetVirtualMachineOperation struct{}

func (o *GetVirtualMachineOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	id := args[0]

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.VirtualMachines.Get(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug && response.VirtualMachine != nil && response.VirtualMachine.Data != nil {
		vm := VirtualMachine{VirtualMachineAttributes: *response.VirtualMachine.Data}
		utils.Render(vm.GetData())
	}

	return nil
}
