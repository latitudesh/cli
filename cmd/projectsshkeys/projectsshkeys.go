// Package projectsshkeys implements the project-scoped SSH key commands
// (`lsh projects ssh-keys`). It reuses the renderable SSHKey model from the
// account-scoped sshkeys package, since both operate on the same resource.
package projectsshkeys

import (
	"github.com/spf13/cobra"
)

// projectID returns the project scope for the command: the --project flag,
// which the root PersistentPreRunE resolves from the flag, $LSH_PROJECT, or an
// interactive prompt before RunE executes.
func projectID(cmd *cobra.Command) string {
	project, _ := cmd.Flags().GetString("project")
	return project
}

// registerProjectFlag adds the --project flag that the shared project-resolution
// hook keys off of. Omitting it defaults to the active project.
func registerProjectFlag(cmd *cobra.Command) {
	cmd.Flags().String("project", "", "Project ID or slug (defaults to the active project)")
}
