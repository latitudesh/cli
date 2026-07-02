package storage_filesystems

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	o := UpdateFilesystemOperation{}
	cmd := &cobra.Command{
		Long:    "Update a filesystem. Currently only the size can be changed.\n",
		RunE:    o.run,
		Short:   "Update a filesystem",
		Example: `  lsh storage-filesystems update fs_xxxxxxxx --size 3000`,
		Use:     "update <id>",
		Args:    cobra.ExactArgs(1),
	}

	cmd.Flags().Int64("size", 0, "New size in GB")

	return cmd
}

type UpdateFilesystemOperation struct{}

// buildUpdateRequest builds the SDK patch body. The ID is a path param and is
// validated by run; only size is patchable today. Split out for unit testing.
func buildUpdateRequest(sizeSet bool, size int64) operations.PatchStorageFilesystemsFilesystemStorageRequestBody {
	attributes := operations.PatchStorageFilesystemsFilesystemStorageAttributes{}
	if sizeSet {
		attributes.SizeInGb = &size
	}

	return operations.PatchStorageFilesystemsFilesystemStorageRequestBody{
		Data: operations.PatchStorageFilesystemsFilesystemStorageData{
			Type:       operations.PatchStorageFilesystemsFilesystemStorageTypeFilesystems,
			Attributes: attributes,
		},
	}
}

func (o *UpdateFilesystemOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	sizeSet := cmd.Flags().Changed("size")
	size, _ := cmd.Flags().GetInt64("size")
	if !sizeSet {
		err := fmt.Errorf("--size is required (the only updatable field)")
		utils.PrintError(err)
		return err
	}

	request := buildUpdateRequest(sizeSet, size)
	// The API needs the ID inside the body too; mirror the path param.
	request.Data.ID = id

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.FilesystemStorage.UpdateFilesystem(ctx, id, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object != nil && response.Object.Data != nil {
		if !lsh.Debug {
			lshFilesystem := Filesystem{FilesystemData: *response.Object.Data}
			utils.RenderStatic(lshFilesystem.GetData())
		}
	}

	return nil
}
