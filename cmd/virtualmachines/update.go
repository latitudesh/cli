package virtualmachines

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	op := UpdateVirtualMachineOperation{}
	cmd := &cobra.Command{
		Long:  "Update a Virtual Machine's name or tags.\n",
		RunE:  op.run,
		Short: "Update a virtual machine",
		Example: `  lsh virtual-machines update vm_xxxxxxxx --name new-name
  lsh virtual-machines update vm_xxxxxxxx --tags tag_xxxx,tag_yyyy`,
		Use:  "update <id>",
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().String("name", "", "New display name (hostname) for the VM")
	cmd.Flags().StringSlice("tags", nil, "Tag IDs to assign (replaces all existing tags)")

	return cmd
}

type UpdateVirtualMachineOperation struct{}

// buildUpdateRequest assembles the update payload from flags. Split out for
// testing without a network call.
func buildUpdateRequest(cmd *cobra.Command, id string) (components.VirtualMachineUpdatePayload, error) {
	if id == "" {
		return components.VirtualMachineUpdatePayload{}, fmt.Errorf("a virtual machine ID is required")
	}
	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("tags") {
		return components.VirtualMachineUpdatePayload{}, fmt.Errorf("nothing to update: pass --name and/or --tags")
	}

	attrs := components.VirtualMachineUpdatePayloadAttributes{}
	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		attrs.Name = &name
	}
	if cmd.Flags().Changed("tags") {
		attrs.Tags, _ = cmd.Flags().GetStringSlice("tags")
	}

	return components.VirtualMachineUpdatePayload{
		Data: components.VirtualMachineUpdatePayloadData{
			Type:       components.VirtualMachineUpdatePayloadTypeVirtualMachines,
			ID:         &id,
			Attributes: attrs,
		},
	}, nil
}

func (o *UpdateVirtualMachineOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]
	request, err := buildUpdateRequest(cmd, id)
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

	response, err := client.VirtualMachines.UpdateVirtualMachine(ctx, id, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug && response.VirtualMachine != nil && response.VirtualMachine.Data != nil {
		vm := VirtualMachine{VirtualMachineAttributes: *response.VirtualMachine.Data}
		utils.RenderStatic(vm.GetData())
	}

	return nil
}
