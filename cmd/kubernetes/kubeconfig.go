package kubernetes

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewKubeconfigCmd() *cobra.Command {
	op := KubeconfigOperation{}
	cmd := &cobra.Command{
		Long: "Print the kubeconfig for a Kubernetes cluster to stdout.\n\n" +
			"Redirect it to a file to use it with kubectl:\n" +
			"  lsh kubernetes kubeconfig kc_xxxxxxxx > kubeconfig.yaml",
		RunE:    op.run,
		Short:   "Print a cluster's kubeconfig",
		Example: `  lsh kubernetes kubeconfig kc_xxxxxxxx > kubeconfig.yaml`,
		Use:     "kubeconfig <cluster-id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type KubeconfigOperation struct{}

func (o *KubeconfigOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	id := args[0]

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.KubernetesClusters.GetKubernetesClusterKubeconfig(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.KubernetesClusterKubeconfig == nil ||
		response.KubernetesClusterKubeconfig.Data == nil ||
		response.KubernetesClusterKubeconfig.Data.Attributes == nil ||
		response.KubernetesClusterKubeconfig.Data.Attributes.Kubeconfig == nil {
		err := fmt.Errorf("no kubeconfig returned for cluster %s (it may still be provisioning)", id)
		utils.PrintError(err)
		return err
	}

	// The kubeconfig is raw YAML meant for redirection into a file, so print it
	// directly to stdout rather than through the table/structured renderer.
	fmt.Print(*response.KubernetesClusterKubeconfig.Data.Attributes.Kubeconfig)

	return nil
}
