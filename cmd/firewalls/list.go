package firewalls

import (
	"context"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListFirewallsOperation{}
	cmd := &cobra.Command{
		Long:  "List all firewalls in the team. Use --project to filter by project.\n",
		RunE:  op.run,
		Short: "List firewalls",
		Example: `  lsh firewalls list
  lsh firewalls list --project my-project
  lsh firewalls list -o json`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Filter by project ID or slug")

	return cmd
}

type ListFirewallsOperation struct{}

func (o *ListFirewallsOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	var filterProject *string
	if cmd.Flags().Changed("project") {
		value, _ := cmd.Flags().GetString("project")
		filterProject = &value
	}

	opts := pagination.Resolve()
	firewalls, result, err := listFirewalls(ctx, client, filterProject, opts)
	if err != nil {
		utils.PrintError(err)
		return err
	}
	if opts.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}

	if !lsh.Debug {
		utils.Render(firewalls.GetData())
	}

	return nil
}

// listFirewalls walks every page of GET /firewalls (the API returns 20 per
// page by default), honoring --page-size/--max-items/--no-paginate.
func listFirewalls(ctx context.Context, client *latitudeshgosdk.Latitudesh, filterProject *string, opts pagination.Options) (Firewalls, pagination.Result, error) {
	firewalls := Firewalls{}
	pageNumber := int64(1)
	first, err := client.Firewalls.List(ctx, filterProject, &opts.PageSize, &pageNumber, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return firewalls, pagination.Result{}, err
	}
	result, err := pagination.Walk(first, opts,
		func(r *operations.ListFirewallsResponse) func() (*operations.ListFirewallsResponse, error) {
			return r.Next
		},
		func(r *operations.ListFirewallsResponse, limit int) int {
			if r.Firewalls == nil {
				return 0
			}
			added := 0
			for i := range r.Firewalls.Data {
				if limit >= 0 && added >= limit {
					break
				}
				firewalls.Data = append(firewalls.Data, &Firewall{FirewallData: r.Firewalls.Data[i]})
				added++
			}
			return added
		},
	)
	return firewalls, result, err
}
