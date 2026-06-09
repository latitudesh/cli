package cli

import "github.com/spf13/cobra"

// helpTopicsGroupID is the Cobra group used to gather long-form help
// commands under a dedicated section of `lsh --help`.
const helpTopicsGroupID = "help-topics"

// newHelpTopic returns a Cobra command that exists purely to host
// long-form documentation. Running it (with or without `help`) prints
// the topic content; it has no subcommands and no side effects.
func newHelpTopic(use, short, long string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		GroupID: helpTopicsGroupID,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}
	// Topics are pure docs — strip the auto-generated Usage/Flags/Global
	// Flags sections so the page reads as a static page rather than as
	// a Cobra command help screen.
	cmd.SetHelpTemplate("{{.Long}}\n")
	cmd.SilenceUsage = true
	return cmd
}

func makeHelpAuthenticationCmd() *cobra.Command {
	return newHelpTopic(
		"authentication",
		"How to sign in: tokens, profiles, env vars",
		`lsh — Authentication

There are two ways to sign in:

  # 1. Browser-assisted (recommended)
  lsh login

  # 2. Existing token (for scripts and CI)
  lsh login --with-token ak_xxxxxxxxxxxxxxxx

After signing in, the credential is stored as a profile in
~/.config/lsh/config.json. Each profile is bound to one team.

  # Switch the active profile
  lsh profile list
  lsh profile use <profile-name>

  # One-shot override for a single command
  lsh --profile <name> servers list

  # Inspect the active context and validate stored tokens
  lsh auth status
  lsh auth status --check

  # Environment overrides
  LATITUDESH_TOKEN=ak_xxx lsh servers list        # bypass any profile
  LSH_PROFILE=acme lsh servers list               # use the 'acme' profile
  LSH_PROJECT=proj_xyz lsh servers list           # pre-fill --project

  # Sign out
  lsh auth logout                                  # active profile
  lsh auth logout --all                            # every profile
`,
	)
}

func makeHelpProfilesCmd() *cobra.Command {
	return newHelpTopic(
		"profiles",
		"How profiles map to teams",
		`lsh — Profiles

A profile is the local identity that binds you to one team. If you are
a member of multiple teams, run 'lsh login' once per team — each login
creates or refreshes a profile named after the team's slug.

  # List the profiles you are logged into
  lsh profile list

  Output:
    PROFILE              TEAM                           EMAIL
    acme *               Acme Inc. (acme)               you@latitude.sh
    labs                 Labs (labs)                    you@latitude.sh

    * = active profile

  # Make a different profile active
  lsh profile use labs

  # Use a profile for a single command (does not change the default)
  lsh --profile labs servers list
`,
	)
}

func makeHelpAutomationCmd() *cobra.Command {
	return newHelpTopic(
		"automation",
		"Run lsh non-interactively (CI, scripts, AI agents)",
		`lsh — Automation

For scripts, CI pipelines and AI agents, run lsh in a way that never
asks for input and always returns deterministic errors when it would
otherwise prompt.

  # Bypass any stored profile with a token from the environment
  LATITUDESH_TOKEN=ak_xxx lsh servers list --project=<project>

  # Disable every interactive prompt — fail fast instead
  lsh --no-input servers list --project=<project>

  # Pre-fill --project from the environment (no prompt)
  LSH_PROJECT=<project> lsh servers list

  # List commands accept --all-projects to skip the prompt entirely
  lsh --no-input servers list --all-projects

When --no-input is set (or stdin is not a TTY, e.g. when piping output),
commands that would otherwise ask for a project return an actionable
error: "--project is required ...". The caller can recover by listing
projects first and retrying:

  lsh --no-input projects list -o json
`,
	)
}

func makeHelpOutputFormatsCmd() *cobra.Command {
	return newHelpTopic(
		"output-formats",
		"Render results as table or JSON",
		`lsh — Output formats

By default, lsh prints a human-readable table:

  lsh servers list --project=my-project

Use --output (or -o) to switch:

  lsh servers list --project=my-project --output=json     # raw JSON
  lsh servers list --project=my-project -o table          # default
`,
	)
}
