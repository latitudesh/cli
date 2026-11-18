package virtualmachines

import (
	"context"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/pagination"
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

	// The endpoint paginates and the SDK exposes a Next closure, so drive the
	// shared pagination.Walk like the other list commands.
	opts := pagination.Resolve()
	request := operations.IndexVirtualMachineRequest{
		FilterProject: filterProject,
		FilterTags:    filterTags,
		PageSize:      &opts.PageSize,
	}

	first, err := client.VirtualMachines.List(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	vms := VirtualMachines{}
	result, err := pagination.Walk(first, opts,
		func(r *operations.IndexVirtualMachineResponse) func() (*operations.IndexVirtualMachineResponse, error) {
			return r.Next
		},
		func(r *operations.IndexVirtualMachineResponse, limit int) pagination.PageInfo {
			if r.VirtualMachines == nil {
				return pagination.PageInfo{}
			}
			data := r.VirtualMachines.Data
			n := len(data)
			if limit >= 0 && n > limit {
				n = limit
			}
			for i := 0; i < n; i++ {
				vms.Data = append(vms.Data, &VirtualMachine{VirtualMachineAttributes: data[i]})
			}
			return pagination.PageInfo{Added: n, Len: len(data), Unread: len(data) - n}
		},
	)
	if err != nil {
		utils.PrintError(err)
		return err
	}
	if opts.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}

	if !lsh.Debug {
		utils.Render(vms.GetData())
	}

	return nil
}
