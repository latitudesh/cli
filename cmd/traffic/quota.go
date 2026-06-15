package traffic

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewQuotaCmd() *cobra.Command {
	op := QuotaTrafficOperation{}
	cmd := &cobra.Command{
		Long:  "Show the traffic quota of each project, per region.\n",
		RunE:  op.run,
		Short: "Show traffic quota",
		Example: `  lsh traffic quota
  lsh traffic quota --project my-project`,
		Use:         "quota",
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Project ID or slug to filter by")

	return cmd
}

type QuotaTrafficOperation struct{}

func (o *QuotaTrafficOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	var filterProject *string
	if project, _ := cmd.Flags().GetString("project"); project != "" {
		filterProject = &project
	}

	response, err := client.Traffic.GetQuota(ctx, filterProject, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		utils.Render(flattenQuota(response.TrafficQuota))
	}

	return nil
}
