package virtualmachines

import (
	"context"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListVirtualMachineOperation{}
	cmd := &cobra.Command{
		Long:  "List all Virtual Machines in the team, optionally filtered by project or tags.\n",
		RunE:  op.run,
		Short: "List virtual machines",
		Example: `  lsh virtual-machines list
  lsh virtual-machines list --project my-project
  lsh virtual-machines list --tags tag_xxxxxxxx -o json`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Filter by project ID or slug")
	cmd.Flags().StringSlice("tags", nil, "Filter by tag IDs (repeatable or comma-separated)")

	return cmd
}

type ListVirtualMachineOperation struct{}

func (o *ListVirtualMachineOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	var filterProject, filterTags *string
	if cmd.Flags().Changed("project") {
		v, _ := cmd.Flags().GetString("project")
		filterProject = &v
	}
	if cmd.Flags().Changed("tags") {
		v, _ := cmd.Flags().GetStringSlice("tags")
		joined := strings.Join(v, ",")
		filterTags = &joined
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.VirtualMachines.List(ctx, filterProject, filterTags, nil, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	vms := VirtualMachines{}
	if response.VirtualMachines != nil {
		data := response.VirtualMachines.Data
		for i := range data {
			vms.Data = append(vms.Data, &VirtualMachine{VirtualMachineAttributes: data[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(vms.GetData())
	}

	return nil
}
