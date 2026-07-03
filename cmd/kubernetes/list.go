package kubernetes

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListClusterOperation{}
	cmd := &cobra.Command{
		Long: "List all Kubernetes clusters in a project.\n\n" +
			"This endpoint accepts the project ID only (proj_xxx), not the slug.",
		RunE:  op.run,
		Short: "List Kubernetes clusters",
		Example: `  lsh kubernetes clusters list --project proj_xxxxxxxx
  lsh kubernetes clusters list --project proj_xxxxxxxx -o json`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	// The list endpoint requires a project scope; --project is resolved by the
	// root command's project-flag flow when omitted.
	cmd.Flags().String("project", "", "Project ID (proj_xxx; the endpoint does not resolve slugs)")

	return cmd
}

type ListClusterOperation struct{}

func (o *ListClusterOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	project, _ := cmd.Flags().GetString("project")

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.KubernetesClusters.ListKubernetesClusters(ctx, project, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	clusters := ClusterList{}
	if response.KubernetesClusters != nil {
		data := response.KubernetesClusters.Data
		for i := range data {
			clusters.Data = append(clusters.Data, &ClusterSummary{KubernetesClusterSummaryData: data[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(clusters.GetData())
	}

	return nil
}
