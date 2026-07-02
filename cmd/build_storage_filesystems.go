package cmd

import (
	storage_filesystems "github.com/latitudesh/lsh/cmd/storage_filesystems"
	cobra "github.com/spf13/cobra"
)

func init() {
	storageFilesystemsCmd.AddCommand(storage_filesystems.NewListCmd())
	storageFilesystemsCmd.AddCommand(storage_filesystems.NewUpdateCmd())
	storageFilesystemsCmd.AddCommand(storage_filesystems.NewDeleteCmd())

	rootCmd.AddCommand(storageFilesystemsCmd)
}

var storageFilesystemsCmd = &cobra.Command{
	Use:   "storage-filesystems",
	Short: "Manage filesystem storage",
	Long:  "Manage filesystem storage: list, resize and delete filesystems.",
	Example: `  lsh storage-filesystems list --project my-project
  lsh storage-filesystems update fs_xxxxxxxx --size 3000
  lsh storage-filesystems delete fs_xxxxxxxx`,
}
