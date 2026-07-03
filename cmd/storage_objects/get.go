package storage_objects

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetBucketOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single object storage bucket by ID.\n",
		RunE:    op.run,
		Short:   "Get a bucket",
		Example: `  lsh storage-objects get bucket_xxxxxxxx`,
		Use:     "get <id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type GetBucketOperation struct{}

func (o *GetBucketOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.ObjectStorage.GetStorageBucket(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object != nil && response.Object.Data != nil {
		if !lsh.Debug {
			bucket := Bucket{ObjectStorageData: *response.Object.Data}
			utils.Render(bucket.GetData())
		}
	}

	return nil
}
