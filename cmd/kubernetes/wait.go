package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	"github.com/spf13/cobra"
)

// waitForCluster blocks until the cluster reaches its Provisioned phase, enters
// a Failed phase, times out, or the user cancels — but only when --wait was
// passed. Progress and the final state go to stderr/stdout so they never
// corrupt structured (-o json) output.
func waitForCluster(cmd *cobra.Command, clusterID string) error {
	o := wait.OptionsFrom(cmd)
	if !o.Enabled {
		if cmd.Flags().Changed("timeout") {
			fmt.Fprintln(os.Stderr, "warning: --timeout has no effect without --wait")
		}
		return nil
	}
	if clusterID == "" {
		fmt.Fprintln(os.Stderr, "warning: --wait ignored: could not determine the cluster ID")
		return nil
	}

	client := lsh.NewClient()
	ctx, cancel := wait.SignalContext(context.Background())
	defer cancel()

	fmt.Fprintf(os.Stderr, "Waiting for cluster %s to finish provisioning… (Ctrl+C to stop)\n", clusterID)

	want := []components.KubernetesClusterDataPhase{components.KubernetesClusterDataPhaseProvisioned}
	fail := []components.KubernetesClusterDataPhase{components.KubernetesClusterDataPhaseFailed}

	phase, err := wait.ForKubernetesClusterPhase(ctx, client, clusterID, want, fail, o, operations.WithRetries(lsh.RetryConfig()))
	switch {
	case errors.Is(err, wait.ErrCanceled):
		return fmt.Errorf("wait canceled (last phase: %s)", phase)
	case errors.Is(err, wait.ErrTimeout):
		return fmt.Errorf("timed out waiting for cluster %s (last phase: %s)", clusterID, phase)
	case errors.Is(err, wait.ErrClusterFailed):
		return fmt.Errorf("cluster %s entered a failed phase: %s", clusterID, phase)
	case err != nil:
		return err
	}

	fmt.Fprintf(os.Stderr, "Cluster %s is now %q\n", clusterID, phase)

	if !lsh.Debug {
		resp, err := client.KubernetesClusters.GetKubernetesCluster(ctx, clusterID, operations.WithRetries(lsh.RetryConfig()))
		if err == nil && resp.KubernetesCluster != nil && resp.KubernetesCluster.Data != nil {
			cluster := Cluster{KubernetesClusterData: *resp.KubernetesCluster.Data}
			utils.RenderStatic(cluster.GetData())
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not fetch the final cluster state for display: %v\n", err)
		}
	}
	return nil
}
