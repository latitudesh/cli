package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/prompt"
	"github.com/latitudesh/lsh/internal/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ProjectOptionalAnnotation marks commands whose --project flag is an
// optional filter rather than a required scope, opting them out of the
// resolution flow below.
const ProjectOptionalAnnotation = "lsh_project_optional"

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
// Commands without a "project" flag are left alone, as are commands that
// declare the ProjectOptionalAnnotation — there --project is an optional
// filter (e.g. events/traffic), not a required scope. The --no-input path
// gives AI agents and other scripted callers a deterministic error to
// recover from instead of a bubbletea prompt they cannot drive.
func resolveProjectFlag(cmd *cobra.Command) error {
	projectFlag := cmd.Flags().Lookup("project")
	if projectFlag == nil {
		return nil
	}
	if cmd.Annotations[ProjectOptionalAnnotation] == "true" {
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

// PickProjectForList resolves the project scope for a "list" command that can
// also run across every project. Precedence: an explicit --project flag,
// LSH_PROJECT, --all-projects, then — in an interactive terminal — a project
// picker that includes an "All projects" entry.
//
// A non-interactive session (or --no-input) has no picker to show, so it keeps
// the behaviour these listings always had and covers every project. Failing
// there instead would break existing scripts (including the ones calling the
// legacy `storage-objects list`) for a prompt they could never have answered.
//
// It returns the chosen project (id or slug, empty when "all") and whether the
// user opted into all projects. The command owns the flags "project",
// "all-projects" and "no-input".
func PickProjectForList(cmd *cobra.Command) (project string, allProjects bool, err error) {
	if v, _ := cmd.Flags().GetString("project"); v != "" {
		return v, false, nil
	}
	if env := os.Getenv("LSH_PROJECT"); env != "" {
		return env, false, nil
	}
	if all, _ := cmd.Flags().GetBool("all-projects"); all {
		return "", true, nil
	}
	noInput, _ := cmd.Flags().GetBool("no-input")
	if noInput || !isInteractive() {
		return "", true, nil
	}
	token := viper.GetString("Authorization")
	if token == "" {
		return "", false, exitcode.Errorf(exitcode.Credentials, "not logged in — run 'lsh login' first")
	}
	selected, err := prompt.SelectProject(cmd.Context(), newAuthClient(), token, true)
	if err != nil {
		return "", false, err
	}
	if selected == prompt.AllProjectsSentinel {
		return "", true, nil
	}
	return selected, false, nil
}
