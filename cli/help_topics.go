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
		"Render results as table, JSON, YAML or CSV (with JMESPath queries)",
		`lsh — Output formats

By default, lsh prints a human-readable table. Use --output (or -o) to switch
to a machine-readable format for scripts, pipelines and AI agents:

  lsh servers list -o table          # default, human-readable
  lsh servers list -o json           # raw JSON
  lsh servers list -o yaml           # YAML
  lsh servers list -o csv            # CSV (header + one row per item)
  lsh servers list --json            # shortcut for -o json

  # Set a per-user default without passing -o every time.
  # Precedence: --output flag > LSH_OUTPUT > config file > default (table)
  export LSH_OUTPUT=json
  lsh servers list                   # prints JSON

  # Force the legacy plain-ASCII table (e.g. CI that parses fixed columns).
  # An explicit -o json/yaml/csv still wins over this.
  LSH_CLASSIC_OUTPUT=true lsh servers list

Filtering with --query (JMESPath)

The --query flag post-processes structured output (json/yaml/csv) with a
JMESPath expression — no jq or extra tooling required:

  # Only the IDs of servers that are powered on
  lsh servers list --query '[?status==`+"`on`"+`].id' -o json

  # A projection of selected fields
  lsh servers list --query '[].{id: id, host: hostname}' -o yaml

--query requires a structured format; combine it with -o json, yaml or csv.

Pagination

List commands fetch every page by default. These flags give you control:

  --page-size N     items requested per API page (default 100)
  --max-items N     stop after N items across all pages (0 = no limit)
  --no-paginate     fetch only the first page; if more exist, the next page
                    number is printed to stderr so you can resume

  lsh servers list --page-size 10 --max-items 50    # at most 50 items, 5 calls
  lsh servers list --no-paginate -o json            # first page only
`,
	)
}
