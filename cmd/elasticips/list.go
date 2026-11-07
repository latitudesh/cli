package elasticips

import (
	"context"
	"fmt"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListElasticIPsOperation{}
	cmd := &cobra.Command{
		Long:  "List all Elastic IPs in the team. Filter with --project, --server or --status.\n",
		RunE:  op.run,
		Short: "List Elastic IPs",
		Example: `  lsh elastic-ips list
  lsh elastic-ips list --project my-project
  lsh elastic-ips list --status active -o json`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Filter by project ID or slug")
	cmd.Flags().String("server", "", "Filter by server ID")
	cmd.Flags().String("status", "", "Filter by status (configuring, active, moving, releasing, error)")

	return cmd
}

type ListElasticIPsOperation struct{}

// validStatuses are the Elastic IP statuses accepted by --status.
var validStatuses = map[string]operations.FilterStatus{
	"configuring": operations.FilterStatusConfiguring,
	"active":      operations.FilterStatusActive,
	"moving":      operations.FilterStatusMoving,
	"releasing":   operations.FilterStatusReleasing,
	"error":       operations.FilterStatusError,
}

// buildListRequest assembles the SDK request from flags. Split out for testing.
func buildListRequest(cmd *cobra.Command) (operations.ListElasticIpsRequest, error) {
	request := operations.ListElasticIpsRequest{}

	if cmd.Flags().Changed("project") {
		value, _ := cmd.Flags().GetString("project")
		request.FilterProject = &value
	}
	if cmd.Flags().Changed("server") {
		value, _ := cmd.Flags().GetString("server")
		request.FilterServer = &value
	}
	if cmd.Flags().Changed("status") {
		value, _ := cmd.Flags().GetString("status")
		status, ok := validStatuses[value]
		if !ok {
			return request, fmt.Errorf("invalid --status %q: must be one of configuring, active, moving, releasing, error", value)
		}
		request.FilterStatus = &status
	}

	return request, nil
}

func (o *ListElasticIPsOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request, err := buildListRequest(cmd)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	opts := pagination.Resolve()
	ips, result, err := listElasticIPs(ctx, client, request, opts)
	if err != nil {
		utils.PrintError(err)
		return err
	}
	if opts.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}

	if !lsh.Debug {
		utils.Render(ips.GetData())
	}

	return nil
}

// listElasticIPs walks every page of GET /elastic_ips (the API returns 20 per
// page by default), honoring --page-size/--max-items/--no-paginate.
func listElasticIPs(ctx context.Context, client *latitudeshgosdk.Latitudesh, request operations.ListElasticIpsRequest, opts pagination.Options) (ElasticIPs, pagination.Result, error) {
	ips := ElasticIPs{}
	pageNumber := int64(1)
	request.PageSize = &opts.PageSize
	request.PageNumber = &pageNumber

	first, err := client.ElasticIps.ListElasticIps(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return ips, pagination.Result{}, err
	}
	result, err := pagination.Walk(first, opts,
		func(r *operations.ListElasticIpsResponse) func() (*operations.ListElasticIpsResponse, error) {
			return r.Next
		},
		func(r *operations.ListElasticIpsResponse, limit int) int {
			if r.ElasticIps == nil {
				return 0
			}
			added := 0
			for i := range r.ElasticIps.Data {
				if limit >= 0 && added >= limit {
					break
				}
				ips.Data = append(ips.Data, &ElasticIP{ElasticIPData: r.ElasticIps.Data[i]})
				added++
			}
			return added
		},
	)
	return ips, result, err
}
