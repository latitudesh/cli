// Package projectuserdata implements the project-scoped user data commands
// (`lsh projects user-data`). It reuses the renderable UserData model from the
// account-scoped userdata package, since both operate on the same resource.
package projectuserdata

import (
	"encoding/base64"

	cobra "github.com/spf13/cobra"
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

// resolveContent mirrors the account-scoped userdata content handling: --content
// takes plain text and is base64-encoded before sending, --content-base64 is
// passed through unchanged.
func resolveContent(cmd *cobra.Command) (string, bool) {
	if cmd.Flags().Changed("content-base64") {
		v, _ := cmd.Flags().GetString("content-base64")
		return v, true
	}
	if cmd.Flags().Changed("content") {
		v, _ := cmd.Flags().GetString("content")
		return base64.StdEncoding.EncodeToString([]byte(v)), true
	}
	return "", false
}

func registerContentFlags(cmd *cobra.Command) {
	cmd.Flags().String("content", "", "Plain-text content (base64-encoded before sending)")
	cmd.Flags().String("content-base64", "", "Already base64-encoded content (sent as-is)")
}
