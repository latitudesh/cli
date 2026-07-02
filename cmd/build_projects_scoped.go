package cmd

import (
	"fmt"
	"os"

	projectsshkeys "github.com/latitudesh/lsh/cmd/projectsshkeys"
	projectuserdata "github.com/latitudesh/lsh/cmd/projectuserdata"
	cobra "github.com/spf13/cobra"
)

// newProjectSSHKeysCmd builds the `projects ssh-keys` group: project-scoped SSH
// key management (list/get/create/delete), built on the Go SDK.
func newProjectSSHKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-keys",
		Short: "Manage a project's SSH keys",
		Long: "Manage the SSH keys scoped to a project.\n\n" +
			"Commands default to the active project; pass --project to override.\n" +
			"To manage team-level keys, use `lsh ssh-keys`.",
		Example: `  lsh projects ssh-keys list
  lsh projects ssh-keys create --name laptop --public-key "ssh-ed25519 AAAA..."
  lsh projects ssh-keys delete ssh_xxxxxxxx --project my-project`,
	}

	cmd.AddCommand(projectsshkeys.NewListCmd())
	cmd.AddCommand(projectsshkeys.NewGetCmd())
	cmd.AddCommand(projectsshkeys.NewCreateCmd())
	cmd.AddCommand(projectsshkeys.NewDeleteCmd())

	return cmd
}

// newProjectUserDataCmd builds the `projects user-data` group: project-scoped
// user data CRUD, built on the Go SDK.
func newProjectUserDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user-data",
		Short: "Manage a project's user data",
		Long: "Manage the user data scoped to a project (project scope).\n\n" +
			"Commands default to the active project; pass --project to override.\n" +
			"To manage team-level user data, use `lsh user-data`.",
		Example: `  lsh projects user-data list
  lsh projects user-data create --description cloud-init --content "#cloud-config"
  lsh projects user-data delete ud_xxxxxxxx --project my-project`,
	}

	cmd.AddCommand(projectuserdata.NewListCmd())
	cmd.AddCommand(projectuserdata.NewGetCmd())
	cmd.AddCommand(projectuserdata.NewCreateCmd())
	cmd.AddCommand(projectuserdata.NewUpdateCmd())
	cmd.AddCommand(projectuserdata.NewDeleteCmd())

	return cmd
}

// wireProjectScopedCommands attaches the SDK-based project-scoped groups to the
// existing (generated) `projects` command. It must run after MakeRootCmd has
// built the projects group, so it is invoked from Execute rather than init().
func wireProjectScopedCommands(root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Name() == "projects" {
			c.AddCommand(newProjectSSHKeysCmd())
			c.AddCommand(newProjectUserDataCmd())
			return
		}
	}
	// The generated `projects` group is the anchor for these subcommands; if it
	// is ever renamed or removed they would silently vanish from the CLI.
	fmt.Fprintln(os.Stderr, "warning: 'projects' command group not found; ssh-keys/user-data project subcommands not registered")
}
