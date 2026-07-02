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

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags",
	Long: "Manage the team's tags.\n\n" +
		"Tags are attached to resources through the resource's own update command,\n" +
		"not through the tag itself.",
	Example: `  lsh tags create --name production --color "#FF0000"
  lsh servers update --id sv_xxxxxxxx --tags tag_xxxxxxxx
  lsh projects update --id my-project --tags tag_xxxxxxxx`,
}
