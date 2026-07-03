package storage_filesystems

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListFilesystemsOperation{}
	cmd := &cobra.Command{
		Long:  "List filesystems in a project. Use --project to filter.\n",
		RunE:  op.run,
		Short: "List filesystems",
		Example: `  lsh storage-filesystems list --project my-project
  lsh storage-filesystems list
  lsh storage-filesystems list -o json`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Project ID or slug to filter by")

	return cmd
}

type ListFilesystemsOperation struct{}

func (o *ListFilesystemsOperation) run(cmd *cobra.Command, args []string) error {
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

	response, err := client.FilesystemStorage.ListFilesystems(ctx, filterProject, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	filesystems := Filesystems{}
	if response.Filesystems != nil {
		for i := range response.Filesystems.Data {
			filesystems.Data = append(filesystems.Data, &Filesystem{FilesystemData: response.Filesystems.Data[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(filesystems.GetData())
	}

	return nil
}
