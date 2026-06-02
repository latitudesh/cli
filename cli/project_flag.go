package cli

import (
	"errors"
	"os"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/latitudesh/lsh/internal/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// resolveProjectFlag ensures the active command has a project value
// when it needs one. The resolution order is:
//
//  1. user passed --project explicitly         → use it
//  2. $LSH_PROJECT is set                      → use it
//  3. command supports --all-projects and it's set → skip (no filter)
//  4. interactive TTY                          → prompt and pick one
//  5. otherwise                                → fail with an actionable error
//
// Commands without a "project" flag are left alone.
func resolveProjectFlag(cmd *cobra.Command) error {
	projectFlag := cmd.Flags().Lookup("project")
	if projectFlag == nil {
		return nil
	}
	if projectFlag.Changed {
		return nil
	}

	if env := os.Getenv("LSH_PROJECT"); env != "" {
		return cmd.Flags().Set("project", env)
	}

	if all := cmd.Flags().Lookup("all-projects"); all != nil && all.Changed {
		return nil
	}

	if !util.IsTTY() {
		return errors.New("--project is required (pass --project=<id>, --all-projects, or set LSH_PROJECT)")
	}

	token := viper.GetString("Authorization")
	if token == "" {
		return errors.New("not logged in — run `lsh login` first")
	}

	supportsAll := cmd.Flags().Lookup("all-projects") != nil
	client := newAuthClient()

	selected, err := prompt.SelectProject(cmd.Context(), client, token, supportsAll)
	if err != nil {
		return err
	}
	if selected == prompt.AllProjectsSentinel {
		// User picked "All projects" in the prompt → leave the flag
		// unset; the generated command will skip the filter.
		return nil
	}
	return cmd.Flags().Set("project", selected)
}
