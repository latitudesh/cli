package cli

import "github.com/spf13/cobra"

// helpTopicsGroupID is the Cobra group used to gather long-form help
// commands under a dedicated section of `lsh --help`.
const helpTopicsGroupID = "help-topics"

// StorageGroupID groups the storage command groups (s3, storage-filesystems,
// volume) under a "Storage:" heading in `lsh --help`. Commands are added in
// init() before MakeRootCmd registers the group, which Cobra allows as long as
// the group exists by the time Execute runs.
const StorageGroupID = "storage"

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

Confirmations (--yes / --no-input)

Destructive object storage commands (lsh s3 rm --recursive, rb --force,
access-keys delete, lifecycle delete --all) ask for confirmation when run
in a terminal. The contract for automation is:

  --yes        skip the confirmation and proceed
  --no-input   never prompt; a command that would have asked fails with
               exit 7 instead of hanging (stdin not being a TTY has the
               same effect)

Prompts and progress go to stderr; stdout carries only data, so '-o json'
and '-o text' remain pipeable. A declined or refused confirmation never
exits 0. See 'lsh help exit-codes' for the full table.

Object storage credentials

Object commands (lsh s3 cp/ls/rm/stat/presign) authenticate with an S3
access key, not with the API token. In CI, export one of the following:

  LSH_S3_ACCESS_KEY_ID        S3 access key id (both variables are required;
  LSH_S3_SECRET_ACCESS_KEY    setting only one fails with exit 4)
  LSH_S3_ENDPOINT_URL         talk to this S3 endpoint without the API;
                              buckets are then addressed by their backend
                              bucket_name (see 'lsh s3 configure export')
  LSH_S3_SIGNING_REGION       SigV4 signing region when it cannot be derived
                              from the endpoint
  LSH_S3_USE_AWS_ENV=1        opt in to reuse AWS_ACCESS_KEY_ID and
                              AWS_SECRET_ACCESS_KEY (ignored when
                              AWS_SESSION_TOKEN is set)

  # Resolve s3://<display-name> through the API, sign with the env key
  LATITUDESH_TOKEN=ak_xxx LSH_S3_ACCESS_KEY_ID=... LSH_S3_SECRET_ACCESS_KEY=... \
    lsh s3 cp ./dump.sql s3://backups/2026/09/

  # No API at all: endpoint + backend bucket name
  LSH_S3_ENDPOINT_URL=https://s3.us-central-1.storage.sh \
  LSH_S3_ACCESS_KEY_ID=... LSH_S3_SECRET_ACCESS_KEY=... \
    lsh s3 ls s3://backups-7f3a/

  # Hand a scoped key to another job (the secret is printed once)
  lsh s3 access-keys create --bucket backups=rw --name ci -o text \
    --query "[0].secret_access_key" | gh secret set LSH_S3_SECRET_ACCESS_KEY

Precedence: LSH_S3_* environment > --access-key <saved-name> > the best
saved key of the active profile ('lsh s3 configure'). Without any of them
the command exits 4 with the commands that fix it.
`,
	)
}

func makeHelpOutputFormatsCmd() *cobra.Command {
	return newHelpTopic(
		"output-formats",
		"Render results as table, JSON, YAML, CSV or text (with JMESPath queries)",
		`lsh — Output formats

By default, lsh prints a human-readable table. Use --output (or -o) to switch
to a machine-readable format for scripts, pipelines and AI agents:

  lsh servers list -o table          # default, human-readable
  lsh servers list -o json           # raw JSON
  lsh servers list -o yaml           # YAML
  lsh servers list -o csv            # CSV (header + one row per item)
  lsh servers list -o text           # raw values, tab-separated
  lsh servers list --json            # shortcut for -o json

  # Set a per-user default without passing -o every time.
  # Precedence: --output flag > LSH_OUTPUT > config file > default (table)
  export LSH_OUTPUT=json
  lsh servers list                   # prints JSON

  # Force the legacy plain-ASCII table (e.g. CI that parses fixed columns).
  # An explicit -o json/yaml/csv/text still wins over this.
  LSH_CLASSIC_OUTPUT=true lsh servers list

Filtering with --query (JMESPath)

The --query flag post-processes structured output (json/yaml/csv/text)
with a JMESPath expression — no jq or extra tooling required:

  # Only the IDs of servers that are powered on
  lsh servers list --query '[?status==`+"`on`"+`].id' -o json

  # A projection of selected fields
  lsh servers list --query '[].{id: id, host: hostname}' -o yaml

  # A single raw value, ready for another tool (no quotes, no JSON)
  lsh s3 access-keys create --bucket backups --name ci -o text \
    --query "[0].secret_access_key" | gh secret set LSH_S3_SECRET_ACCESS_KEY

--query requires a structured format; combine it with -o json, yaml, csv or
text.

The text format

-o text prints values without quotes or structure:
a scalar on one line; a list of scalars one per line; a list of objects as
one tab-separated row per item with keys in sorted order; a single object
as key<TAB>value lines. Nested values are JSON-encoded so a row never spans
several lines. Use it with --query to extract exactly one field for a shell
variable or a pipe.

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

func makeHelpExitCodesCmd() *cobra.Command {
	return newHelpTopic(
		"exit-codes",
		"Process exit codes for scripts and CI",
		`lsh — Exit codes

The object storage commands ('lsh s3' and its subcommands) attach a
specific exit code to every failure so scripts can tell "not found" from
"no credentials" from "refused for safety" without parsing stderr.

  Code  Meaning
  ----  -----------------------------------------------------------
  0     success (including an empty listing)
  1     generic error, or one or more transfers failed
  2     invalid usage: bad URI or flag, ambiguous bucket,
        --recursive on a whole bucket without --all
  3     not found: bucket, object, access key or lifecycle rule
  4     credentials missing or invalid (no S3 access key,
        InvalidAccessKeyId, SignatureDoesNotMatch)
  5     permission denied (403 from the API or the S3 endpoint)
  6     partial success: some objects failed in rm --recursive
        or rb --force; the remaining ones are listed on stderr
  7     refused for safety: non-empty bucket without --force,
        --max-delete exceeded, object lock retention, prompt
        declined, or a confirmation needed without a TTY and
        without --yes
  130   interrupted (Ctrl-C / SIGINT) after in-flight work was
        aborted

Errors are always printed to stderr; stdout carries only data, so
'-o json' and '-o text' output stays pipeable even when a command fails.

  lsh s3 cp ./dump.sql s3://backups/ || case $? in
    4) echo "configure an access key: lsh s3 configure" ;;
    7) echo "refused; add --yes in CI" ;;
  esac

Older command groups (servers, projects, plans, ...) predate this table
and still exit 1 for every error. Their behaviour is unchanged; only
'lsh s3' uses the codes above.
`,
	)
}
