package cmd

import (
	tags "github.com/latitudesh/lsh/cmd/tags"
	cobra "github.com/spf13/cobra"
)

func init() {
	tagsCmd.AddCommand(tags.NewDestroyCmd())
	tagsCmd.AddCommand(tags.NewUpdateCmd())
	tagsCmd.AddCommand(tags.NewListCmd())
	tagsCmd.AddCommand(tags.NewCreateCmd())

	rootCmd.AddCommand(tagsCmd)
}

var tagsCmd = &cobra.Command{Use: "tags"}
