package storage_objects

import (
	"context"
	"fmt"
	"net/http"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	op := DeleteBucketOperation{}
	cmd := &cobra.Command{
		Long:    "Delete an object storage bucket by ID.\n",
		RunE:    op.run,
		Short:   "Delete a bucket",
		Example: `  lsh storage-objects delete bucket_xxxxxxxx`,
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type DeleteBucketOperation struct{}

func (o *DeleteBucketOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.ObjectStorage.DeleteStorageBuckets(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		// The API answers deletes with 200 or 204 depending on the path.
		status := 0
		if resp.HTTPMeta.Response != nil {
			status = resp.HTTPMeta.Response.StatusCode
		}
		if status == http.StatusOK || status == http.StatusNoContent {
			fmt.Printf("\nBucket deleted successfully!\n")
		} else {
			fmt.Printf("Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
