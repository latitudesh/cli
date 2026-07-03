package kubernetes

import (
	"context"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateClusterOperation{}
	cmd := &cobra.Command{
		Long: "Create a Kubernetes cluster in a project.\n\n" +
			"The project, region (site) and control-plane plan are required. Use " +
			"--wait to block until the cluster is provisioned.",
		RunE:  op.run,
		Short: "Create a Kubernetes cluster",
		Example: `  lsh kubernetes clusters create --project my-project --region SAO2 --plan c2-small-x86
  lsh kubernetes clusters create --project my-project --region SAO2 --plan c2-small-x86 \
    --worker-plan c2-small-x86 --worker-count 3 --kubernetes-version v1.34.3+rke2r1 --wait`,
		Use: "create",
	}

	cmd.Flags().String("name", "", "Cluster name (auto-generated if omitted)")
	cmd.Flags().String("project", "", "Project ID where the cluster is created (required)")
	cmd.Flags().String("region", "", "Region/site code where the cluster is deployed, e.g. SAO2 (required)")
	cmd.Flags().String("plan", "", "Machine plan for control plane nodes (required)")
	cmd.Flags().String("worker-plan", "", "Machine plan for worker nodes (defaults to control-plane plan)")
	cmd.Flags().StringSlice("ssh-keys", nil, "SSH key IDs for node access (repeatable)")
	cmd.Flags().String("kubernetes-version", "", "Kubernetes version to install (defaults to the latest supported)")
	cmd.Flags().Int64("control-plane-count", 0, "Number of control plane nodes (defaults to 1)")
	cmd.Flags().Int64("worker-count", 0, "Number of worker nodes (defaults to 1)")
	cmd.Flags().String("os", "", "Operating system for the nodes")
	wait.AddFlags(cmd)

	return cmd
}

type CreateClusterOperation struct{}

// buildCreateRequest assembles the SDK payload from flags. Split out for
// testing without a network call.
func buildCreateRequest(cmd *cobra.Command) (components.CreateKubernetesCluster, error) {
	project, _ := cmd.Flags().GetString("project")
	region, _ := cmd.Flags().GetString("region")
	plan, _ := cmd.Flags().GetString("plan")

	if project == "" {
		return components.CreateKubernetesCluster{}, fmt.Errorf("--project is required")
	}
	if region == "" {
		return components.CreateKubernetesCluster{}, fmt.Errorf("--region is required")
	}
	if plan == "" {
		return components.CreateKubernetesCluster{}, fmt.Errorf("--plan is required")
	}

	attrs := components.CreateKubernetesClusterAttributes{
		ProjectID: project,
		Site:      region,
		Plan:      plan,
	}

	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		attrs.Name = &name
	}
	if cmd.Flags().Changed("worker-plan") {
		wp, _ := cmd.Flags().GetString("worker-plan")
		attrs.WorkerPlan = &wp
	}
	if cmd.Flags().Changed("ssh-keys") {
		attrs.SSHKeys, _ = cmd.Flags().GetStringSlice("ssh-keys")
	}
	if cmd.Flags().Changed("kubernetes-version") {
		v, _ := cmd.Flags().GetString("kubernetes-version")
		attrs.KubernetesVersion = &v
	}
	if cmd.Flags().Changed("control-plane-count") {
		c, _ := cmd.Flags().GetInt64("control-plane-count")
		attrs.ControlPlaneCount = &c
	}
	if cmd.Flags().Changed("worker-count") {
		c, _ := cmd.Flags().GetInt64("worker-count")
		attrs.WorkerCount = &c
	}
	if cmd.Flags().Changed("os") {
		osSlug, _ := cmd.Flags().GetString("os")
		attrs.OperatingSystem = &osSlug
	}

	return components.CreateKubernetesCluster{
		Data: components.CreateKubernetesClusterData{
			Type:       components.CreateKubernetesClusterTypeKubernetesClusters,
			Attributes: attrs,
		},
	}, nil
}

func (o *CreateClusterOperation) run(cmd *cobra.Command, args []string) error {
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

	response, err := client.KubernetesClusters.CreateKubernetesCluster(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	var clusterID string
	if response.KubernetesClusterCreateResponse != nil && response.KubernetesClusterCreateResponse.Data != nil {
		data := response.KubernetesClusterCreateResponse.Data
		clusterID = getStr(data.ID)
		if !lsh.Debug {
			// stderr keeps stdout clean for structured output (--wait -o json).
			fmt.Fprintf(os.Stderr, "\nKubernetes cluster %s created (status: %s).\n\n", clusterID, getStr(statusOf(data)))
		}
	}

	// Without --wait, stdout must still carry the created resource so
	// -o json pipelines get structured output (the wait path renders the
	// final state itself).
	if !lsh.Debug && !wait.OptionsFrom(cmd).Enabled && clusterID != "" {
		resp, gerr := client.KubernetesClusters.GetKubernetesCluster(ctx, clusterID, operations.WithRetries(lsh.RetryConfig()))
		if gerr == nil && resp.KubernetesCluster != nil && resp.KubernetesCluster.Data != nil {
			cluster := Cluster{KubernetesClusterData: *resp.KubernetesCluster.Data}
			utils.RenderStatic(cluster.GetData())
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not fetch the created cluster for display: %v\n", gerr)
		}
	}

	return waitForCluster(cmd, clusterID)
}

func statusOf(d *components.KubernetesClusterCreateResponseData) *string {
	if d == nil || d.Attributes == nil {
		return nil
	}
	return d.Attributes.Status
}
