package virtualmachines

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateVirtualMachineOperation{}
	cmd := &cobra.Command{
		Long: "Provision a Virtual Machine in a project.\n\n" +
			"The plan and project are required. Use --wait to block until the VM is " +
			"running.",
		RunE:  op.run,
		Short: "Create a virtual machine",
		Example: `  lsh virtual-machines create --plan plan_xxxxxxxx --project my-project
  lsh virtual-machines create --plan plan_xxxxxxxx --project my-project --name my-vm --os ubuntu-24-04
  lsh virtual-machines create --plan plan_xxxxxxxx --project my-project --ssh-keys key_xxxx --wait`,
		Use: "create",
	}

	cmd.Flags().String("name", "", "Name (hostname) for the Virtual Machine")
	cmd.Flags().String("plan", "", "Plan ID (plan_xxx) for the Virtual Machine (required; VM plans have no slug)")
	cmd.Flags().String("project", "", "Project ID or slug where the VM is created (required)")
	cmd.Flags().String("region", "", "Site/region slug where the VM is provisioned (defaults to DAL)")
	cmd.Flags().String("os", "", "Operating system slug (defaults to the plan's default OS)")
	cmd.Flags().StringSlice("ssh-keys", nil, "SSH key IDs to add to the VM (repeatable)")
	cmd.Flags().String("user-data", "", "User data record reference (e.g. ud_xxxx) applied as cloud-init")
	cmd.Flags().StringSlice("tags", nil, "Tag IDs to assign to the VM (repeatable)")
	wait.AddFlags(cmd)

	return cmd
}

type CreateVirtualMachineOperation struct{}

// buildCreateRequest assembles the SDK payload from the command flags. It is
// kept separate from run so it can be unit-tested without a network call.
func buildCreateRequest(cmd *cobra.Command) (components.VirtualMachinePayload, error) {
	plan, _ := cmd.Flags().GetString("plan")
	project, _ := cmd.Flags().GetString("project")
	if plan == "" {
		return components.VirtualMachinePayload{}, fmt.Errorf("--plan is required")
	}
	if project == "" {
		return components.VirtualMachinePayload{}, fmt.Errorf("--project is required")
	}

	attrs := &components.VirtualMachinePayloadAttributes{
		Plan:    &plan,
		Project: &project,
	}

	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		attrs.Name = &name
	}
	if cmd.Flags().Changed("region") {
		region, _ := cmd.Flags().GetString("region")
		attrs.Site = &region
	}
	if cmd.Flags().Changed("os") {
		osSlug, _ := cmd.Flags().GetString("os")
		attrs.OperatingSystem = &osSlug
	}
	if cmd.Flags().Changed("ssh-keys") {
		attrs.SSHKeys, _ = cmd.Flags().GetStringSlice("ssh-keys")
	}
	if cmd.Flags().Changed("tags") {
		attrs.Tags, _ = cmd.Flags().GetStringSlice("tags")
	}
	if cmd.Flags().Changed("user-data") {
		ud, _ := cmd.Flags().GetString("user-data")
		userData := components.CreateVirtualMachinePayloadUserDataStr(ud)
		attrs.UserData = &userData
	}

	typ := components.VirtualMachinePayloadTypeVirtualMachines
	return components.VirtualMachinePayload{
		Data: &components.VirtualMachinePayloadData{
			Type:       &typ,
			Attributes: attrs,
		},
	}, nil
}

func (o *CreateVirtualMachineOperation) run(cmd *cobra.Command, args []string) error {
	request, err := buildCreateRequest(cmd)
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

	response, err := client.VirtualMachines.Create(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	var vmID string
	if response.VirtualMachine != nil && response.VirtualMachine.Data != nil {
		vmID = getStr(response.VirtualMachine.Data.ID)
		if !lsh.Debug {
			vm := VirtualMachine{VirtualMachineAttributes: *response.VirtualMachine.Data}
			utils.RenderStatic(vm.GetData())
		}
	}

	return waitForVirtualMachine(cmd, vmID)
}
