package storage_objects

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListBucketsOperation{}
	cmd := &cobra.Command{
		Long:  "List object storage buckets in the project.\n",
		RunE:  op.run,
		Short: "List buckets",
		Example: `  lsh storage-objects list --project my-project
  lsh storage-objects list`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Project ID or slug to filter by")

	return cmd
}

type ListBucketsOperation struct{}

func (o *ListBucketsOperation) run(cmd *cobra.Command, args []string) error {
	var filterProject *string
	if cmd.Flags().Changed("project") {
		value, _ := cmd.Flags().GetString("project")
		filterProject = &value
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.ObjectStorage.GetStorageBuckets(ctx, filterProject, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	buckets := Buckets{}
	if response.ObjectStorages != nil {
		for i := range response.ObjectStorages.Data {
			buckets.Data = append(buckets.Data, &Bucket{ObjectStorageData: response.ObjectStorages.Data[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(buckets.GetData())
	}

	return nil
}
