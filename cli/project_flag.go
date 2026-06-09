package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/latitudesh/lsh/internal/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// isInteractive reports whether we can prompt the user. It's a package
// var so tests can force the non-interactive path deterministically.
var isInteractive = util.IsTTY

// resolveProjectFlag ensures the active command has a project value
// when it needs one. The resolution order is:
//
//  1. user passed --project explicitly         → use it
//  2. $LSH_PROJECT is set                      → use it
//  3. command supports --all-projects and it's set → skip (no filter)
//  4. --no-input was passed, or stdin is not a TTY → fail with an actionable error
//  5. interactive TTY                          → prompt and pick one
//
// Commands without a "project" flag are left alone. The --no-input path
// gives AI agents and other scripted callers a deterministic error to
// recover from instead of a bubbletea prompt they cannot drive.
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

	supportsAll := cmd.Flags().Lookup("all-projects") != nil
	if supportsAll {
		// Only skip the project requirement when --all-projects is actually
		// true; an explicit --all-projects=false must not bypass it.
		if allProjects, _ := cmd.Flags().GetBool("all-projects"); allProjects {
			return nil
		}
	}

	noInput, _ := cmd.Flags().GetBool("no-input")
	if noInput || !isInteractive() {
		hint := "pass --project=<id> or set LSH_PROJECT"
		if supportsAll {
			hint = "pass --project=<id>, --all-projects, or set LSH_PROJECT"
		}
		return fmt.Errorf("--project is required (%s)", hint)
	}

	token := viper.GetString("Authorization")
	if token == "" {
		return errors.New("not logged in — run `lsh login` first")
	}

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
