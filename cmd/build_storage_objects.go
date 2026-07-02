package cmd

import (
	storage_objects "github.com/latitudesh/lsh/cmd/storage_objects"
	cobra "github.com/spf13/cobra"
)

func init() {
	storageObjectsCmd.AddCommand(storage_objects.NewListCmd())
	storageObjectsCmd.AddCommand(storage_objects.NewGetCmd())
	storageObjectsCmd.AddCommand(storage_objects.NewCreateCmd())
	storageObjectsCmd.AddCommand(storage_objects.NewDeleteCmd())

	rootCmd.AddCommand(storageObjectsCmd)
}

var storageObjectsCmd = &cobra.Command{
	Use:   "storage-objects",
	Short: "Manage object storage buckets",
	Long:  "Manage object storage buckets: list, get, create and delete S3-compatible buckets.",
	Example: `  lsh storage-objects list --project my-project
  lsh storage-objects create --project my-project --name my-bucket --region SAO2
  lsh storage-objects get bucket_xxxxxxxx
  lsh storage-objects delete bucket_xxxxxxxx`,
}
