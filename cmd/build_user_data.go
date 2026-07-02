package cmd

import (
	userdata "github.com/latitudesh/lsh/cmd/userdata"
	cobra "github.com/spf13/cobra"
)

func init() {
	userDataCmd.AddCommand(userdata.NewListCmd())
	userDataCmd.AddCommand(userdata.NewGetCmd())
	userDataCmd.AddCommand(userdata.NewCreateCmd())
	userDataCmd.AddCommand(userdata.NewUpdateCmd())
	userDataCmd.AddCommand(userdata.NewDeleteCmd())

	rootCmd.AddCommand(userDataCmd)
}

// userDataCmd is the account/team-scoped user data group, built on the Go SDK.
// To manage a specific project's user data, use `lsh projects user-data`.
var userDataCmd = &cobra.Command{
	Use:   "user-data",
	Short: "Manage team user data",
	Long: "Manage the team's user data (account scope).\n\n" +
		"User data content is stored base64-encoded; the create/update commands\n" +
		"accept plain text via --content and encode it for you.\n\n" +
		"To manage a specific project's user data, use `lsh projects user-data`.",
	Example: `  lsh user-data list
  lsh user-data create --description cloud-init --content "#cloud-config"
  lsh user-data get ud_xxxxxxxx
  lsh user-data update ud_xxxxxxxx --description renamed
  lsh user-data delete ud_xxxxxxxx`,
}
