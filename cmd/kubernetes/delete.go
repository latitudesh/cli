package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	op := DeleteClusterOperation{}
	cmd := &cobra.Command{
		Long:    "Delete a Kubernetes cluster by its ID.\n",
		RunE:    op.run,
		Short:   "Delete a Kubernetes cluster",
		Example: `  lsh kubernetes clusters delete kc_xxxxxxxx`,
		Use:     "delete <cluster-id>",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type DeleteClusterOperation struct{}

func (o *DeleteClusterOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	id := args[0]

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.KubernetesClusters.DeleteKubernetesCluster(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		status := 0
		if resp.HTTPMeta.Response != nil {
			status = resp.HTTPMeta.Response.StatusCode
		}
		if status == http.StatusOK || status == http.StatusAccepted || status == http.StatusNoContent {
			fmt.Fprintf(os.Stderr, "\nKubernetes cluster %s deletion requested.\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
