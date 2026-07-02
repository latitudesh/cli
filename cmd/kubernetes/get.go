package kubernetes

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetClusterOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single Kubernetes cluster by its ID.\n",
		RunE:    op.run,
		Short:   "Get a Kubernetes cluster",
		Example: `  lsh kubernetes clusters get kc_xxxxxxxx`,
		Use:     "get <cluster-id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type GetClusterOperation struct{}

func (o *GetClusterOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	id := args[0]

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.KubernetesClusters.GetKubernetesCluster(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug && response.KubernetesCluster != nil && response.KubernetesCluster.Data != nil {
		cluster := Cluster{KubernetesClusterData: *response.KubernetesCluster.Data}
		utils.Render(cluster.GetData())
	}

	return nil
}
