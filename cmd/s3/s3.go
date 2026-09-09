// Package s3 implements `lsh s3`, the object storage command group. It mixes
// control-plane calls to the Latitude API (buckets, access keys, lifecycle,
// metrics, usage) with data-plane calls straight to the bucket's S3 endpoint
// (ls, cp, rm, stat, presign), using the verbs and flags of `aws s3` so
// existing habits and scripts carry over.
package s3

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Persistent flag names shared by the data-plane commands.
const (
	flagAccessKey     = "access-key"
	flagEndpointURL   = "endpoint-url"
	flagSigningRegion = "signing-region"
	flagS3Region      = "s3-region" // aws-flavoured alias of --signing-region
	flagAddressing    = "addressing-style"
	flagDryRunAWS     = "dryrun" // aws spelling of --dry-run
	flagProject       = "project"
	flagYes           = "yes"
	flagSite          = "site"
)

// NewGroupCmd builds the `s3` group. Subcommands are attached by
// cmd/build_s3.go so each lives in its own file.
func NewGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "s3",
		Aliases: []string{"buckets", "storage-objects"},
		// Typing these at the root gets a "did you mean s3" hint.
		SuggestFor: []string{"s3api", "upload", "download", "configure", "bucket", "objects", "object-storage"},
		GroupID:    cli.StorageGroupID,
		Short:      "Object storage: buckets, objects, access keys and lifecycle",
		Long: `Manage S3-compatible object storage.

Bucket management, access keys, lifecycle rules and metrics go through the
Latitude API; object operations (ls, cp, rm, stat, presign) talk to the
bucket's S3 endpoint directly. Buckets are addressed as s3://<bucket>[/<key>],
where <bucket> is the display name, the bkt_ ID or the backend bucket name.
Endpoint, signing region and path-style addressing are resolved from the API,
so nothing has to be configured by hand.

Getting started:
  lsh s3 mb s3://backups --region DAL --project my-project
  lsh s3 cp ./dump.sql s3://backups/2026/09/
  lsh s3 ls s3://backups/2026/09/

Exit codes for scripts: 'lsh help exit-codes'.`,
		Example: `  lsh s3 ls
  lsh s3 ls s3://backups/2026/ --recursive --human-readable --summarize
  lsh s3 cp ./dump.sql s3://backups/2026/09/
  lsh s3 cp s3://backups/2026/09/dump.sql ./restore/
  lsh s3 rm s3://backups/tmp/ --recursive --dryrun
  lsh s3 presign s3://backups/report.pdf --expires-in 15m
  lsh s3 access-keys create --bucket backups --name ci-deploy
  lsh s3 lifecycle create s3://logs --prefix tmp/ --expiration-days 7`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if v, _ := cmd.Flags().GetBool(flagDryRunAWS); v {
				lsh.DryRun = true
			}
			return nil
		},
	}

	pf := cmd.PersistentFlags()
	pf.String(flagAccessKey, "", "use the saved access key with this name instead of the automatic selection")
	pf.String(flagEndpointURL, "", "address the bucket on this S3 endpoint without the Latitude API (bucket is the backend name; credentials only from the environment)")
	pf.String(flagSigningRegion, "", "override the SigV4 signing region derived from the endpoint")
	pf.String(flagS3Region, "", "alias of --signing-region")
	pf.String(flagAddressing, objectstorage.AddressingPath, "S3 addressing style: path or virtual")
	pf.Bool(flagDryRunAWS, false, "alias of --dry-run")
	_ = pf.MarkHidden(flagS3Region)
	_ = pf.MarkHidden(flagAddressing)
	_ = pf.MarkHidden(flagDryRunAWS)

	return cmd
}

// newCmd applies the conventions every s3 subcommand shares: usage is never
// echoed on errors. Error printing/exit-code handling is installed by
// Finalize once the whole tree is built (commands may assign RunE/PreRunE
// after construction).
func newCmd(c *cobra.Command) *cobra.Command {
	c.SilenceUsage = true
	return c
}

// Finalize walks the group's command tree and installs the shared error
// contract on every command:
//   - errors raised by cobra itself (bad flag, wrong argument count) are
//     printed once and exit 2;
//   - errors returned by PreRunE/RunE are printed once through printErr (exit
//     code preserved) and cobra is told not to echo them again.
//
// build_s3.go calls it after attaching the subcommands.
func Finalize(root *cobra.Command) {
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.SilenceErrors = true
		return printErr(exitcode.New(exitcode.Usage, err))
	})
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.SilenceUsage = true
		// Mark the subtree so shared validation (cli.MakeRootCmd's pre-run)
		// can apply these exit codes here without changing the older groups.
		if c.Annotations == nil {
			c.Annotations = map[string]string{}
		}
		c.Annotations[exitcode.OptInAnnotation] = "true"
		if args := c.Args; args != nil {
			c.Args = func(cmd *cobra.Command, a []string) error {
				if err := args(cmd, a); err != nil {
					cmd.SilenceErrors = true
					return printErr(exitcode.New(exitcode.Usage, err))
				}
				return nil
			}
		}
		if pre := c.PreRunE; pre != nil {
			c.PreRunE = func(cmd *cobra.Command, a []string) error {
				return printOnce(cmd, pre(cmd, a))
			}
		}
		if run := c.RunE; run != nil {
			c.RunE = func(cmd *cobra.Command, a []string) error {
				return printOnce(cmd, run(cmd, a))
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// printOnce prints err unless the command already did (objectstorage.PrintError
// marks it) and silences cobra so the message is not duplicated; the exit
// code travels with the returned error.
func printOnce(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	if !objectstorage.IsPrinted(err) {
		err = printErr(err)
	}
	cmd.SilenceErrors = true
	return err
}

// addProjectFlag registers --project. optional=true marks it as a filter /
// disambiguator so the root pre-run never prompts for it.
func addProjectFlag(cmd *cobra.Command, optional bool, usage string) {
	cmd.Flags().String(flagProject, "", usage)
	if optional {
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		cmd.Annotations[cli.ProjectOptionalAnnotation] = "true"
	}
}

// addBucketFilterFlags registers the flags that disambiguate a bucket name that
// resolves to several buckets: --storage-class (short -c) and --site. They are
// only useful when a display name is reused across classes or sites; the bkt_
// ID always resolves without them.
func addBucketFilterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(flagStorageClass, "c", "", "disambiguate a repeated bucket name by storage class (standard or high_performance)")
	cmd.Flags().String(flagSite, "", "disambiguate a repeated bucket name by Latitude site (e.g. DAL, TYO4)")
}

// addYesFlag registers --yes/-y.
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP(flagYes, "y", false, "do not ask for confirmation")
}

// projectFlag returns the --project value (or LSH_PROJECT when unset).
func projectFlag(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString(flagProject); v != "" {
		return v
	}
	return os.Getenv("LSH_PROJECT")
}

// profileFlag returns the --profile override.
func profileFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("profile")
	return v
}

// endpointOverride returns --endpoint-url or LSH_S3_ENDPOINT_URL.
func endpointOverride(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString(flagEndpointURL); v != "" {
		return v
	}
	return os.Getenv(objectstorage.EnvEndpointURL)
}

// signingRegionOverride returns --signing-region / --s3-region / env.
func signingRegionOverride(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString(flagSigningRegion); v != "" {
		return v
	}
	if v, _ := cmd.Flags().GetString(flagS3Region); v != "" {
		return v
	}
	return os.Getenv(objectstorage.EnvSigningRegion)
}

// dryRun reports whether --dry-run (or --dryrun) is active.
func dryRun() bool { return lsh.DryRun }

// isHuman reports whether output goes to the human (table) format, in which
// case the aws-style plain lines are printed instead of the table renderer.
func isHuman() bool { return renderer.ResolveFormat() == renderer.FormatTable }

// render prints structured results through the shared renderer without the
// interactive table (object listings can be huge and must stay pipeable).
func render(items []renderer.ResponseData) { renderer.RenderStatic(items) }

// printErr prints a humanized error to stderr and returns it for RunE.
func printErr(err error) error { return objectstorage.PrintError(err) }

// newResolver builds the bucket resolver for a command.
func newResolver(cmd *cobra.Command) *objectstorage.Resolver {
	r := &objectstorage.Resolver{
		Project:        projectFlag(cmd),
		EndpointURL:    endpointOverride(cmd),
		SigningRegion:  signingRegionOverride(cmd),
		RetryOptions:   []operations.Option{operations.WithRetries(lsh.RetryConfig())},
		HasFilterFlags: cmd.Flags().Lookup(flagSite) != nil,
	}
	if raw, _ := cmd.Flags().GetString(flagStorageClass); strings.TrimSpace(raw) != "" {
		class, err := objectstorage.ParseStorageClass(raw)
		if err != nil {
			r.FilterErr = fmt.Errorf("--%s: %w", flagStorageClass, err)
		} else {
			r.ClassFilter = class
		}
	}
	if site, _ := cmd.Flags().GetString(flagSite); strings.TrimSpace(site) != "" {
		r.SiteFilter = strings.TrimSpace(site)
	}
	// Without a token the SDK would send an empty bearer and every API call
	// would come back as 401; leave API nil so callers get the "not logged in"
	// error (exit 4) with the LSH_S3_ENDPOINT_URL alternative instead.
	if r.EndpointURL == "" && viper.GetString("Authorization") != "" {
		r.API = apiClient()
	}
	if r.EndpointURL != "" {
		endpointHintOnce.Do(func() {
			objectstorage.Hintf("note: --endpoint-url set; bypassing the Latitude API and using credentials from the environment only")
		})
	}
	return r
}

// endpointHintOnce makes the endpoint-override note appear once per run even
// when a command builds several resolvers (stat, remote-to-remote copies).
var endpointHintOnce sync.Once

// apiClient returns the Latitude API client for object storage commands (the
// regular client plus the /storage response normalization the current SDK
// needs; see objectstorage.NewAPIClient).
func apiClient() *sdk.Latitudesh { return objectstorage.NewAPIClient() }

// resolveBucket resolves a bucket token (name, bkt_ id or backend name).
func resolveBucket(ctx context.Context, cmd *cobra.Command, token string) (*objectstorage.Bucket, error) {
	return newResolver(cmd).Resolve(ctx, token)
}

// Session bundles everything a data-plane command needs for one bucket.
type Session struct {
	Bucket *objectstorage.Bucket
	Cred   objectstorage.Credential
	Client *minio.Client
}

// openBucket resolves the bucket, selects a credential and builds the S3
// client. write=true requests a credential with write permission.
func openBucket(ctx context.Context, cmd *cobra.Command, token string, write bool) (*Session, error) {
	b, err := resolveBucket(ctx, cmd, token)
	if err != nil {
		return nil, err
	}
	// A high_performance key is bound to one site, but the SDK model drops the
	// bucket's site, so without this the site check in keyMatchesBucket is a
	// no-op on the data plane and a key from another site can be selected.
	// The site is only a selection constraint, so the lookup is skipped when
	// the credential does not come from the profile; when it does, a failed
	// lookup has to fail the command — continuing with an empty site turns the
	// constraint into a wildcard and lets a key from another site win.
	if b.StorageClass == objectstorage.ClassHighPerformance && b.Site == "" && !b.EndpointOverride && usesSavedCredential() {
		if fillErr := newResolver(cmd).FillSite(ctx, b); fillErr != nil {
			return nil, exitcode.Errorf(exitcode.Of(fillErr),
				"could not determine which site bucket %s is in, and a high_performance access key is only valid in its own site: %v\n  retry, or name the key explicitly with --access-key <name>",
				b.Display(), fillErr)
		}
	}
	return openResolved(cmd, b, write)
}

// usesSavedCredential reports whether the credential will come from the active
// profile, which is the only case where the bucket's site changes the outcome.
// LSH_S3_* credentials bypass the profile entirely; an explicit --access-key
// does not, because the named key is still checked against the bucket's site.
func usesSavedCredential() bool {
	_, fromEnv, _ := objectstorage.EnvCredential()
	return !fromEnv
}

// openResolved is openBucket for an already-resolved bucket.
func openResolved(cmd *cobra.Command, b *objectstorage.Bucket, write bool) (*Session, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	accessKey, _ := cmd.Flags().GetString(flagAccessKey)
	cred, err := objectstorage.ResolveCredential(b, objectstorage.CredentialOptions{
		AccessKeyName:   accessKey,
		ProfileOverride: profileFlag(cmd),
		Write:           write,
	})
	if err != nil {
		return nil, err
	}
	addressing, _ := cmd.Flags().GetString(flagAddressing)
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{
		Debug:      lsh.Debug,
		Addressing: addressing,
	})
	if err != nil {
		return nil, err
	}
	if lsh.Debug {
		fmt.Fprintf(os.Stderr, "[s3] bucket=%s backend=%s endpoint=%s signing-region=%s addressing=%s credential=%s\n",
			b.Display(), b.BucketName, b.Endpoint, b.SigningRegion, addressing, cred.Describe(b.ID))
	}
	return &Session{Bucket: b, Cred: cred, Client: client}, nil
}

// humanize maps an S3/API error for a session.
func (s *Session) humanize(err error) error {
	if s == nil {
		return objectstorage.Humanize(err, nil, nil)
	}
	return objectstorage.Humanize(err, s.Bucket, &s.Cred)
}

// unsupportedAWSFlags registers flags that exist in `aws s3` but have no
// equivalent on Latitude, so scripts get an explanation instead of
// "unknown flag". Each entry maps the flag to the reason/alternative.
func unsupportedAWSFlags(cmd *cobra.Command, flags map[string]string) {
	for name := range flags {
		cmd.Flags().String(name, "", "not supported on Latitude object storage")
		_ = cmd.Flags().MarkHidden(name)
	}
	// Bool-style flags must also parse without a value.
	pre := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		for name, why := range flags {
			if c.Flags().Changed(name) {
				return printErr(exitcode.Errorf(exitcode.Usage, "--%s is not supported on Latitude object storage: %s", name, why))
			}
		}
		if pre != nil {
			return pre(c, args)
		}
		return nil
	}
}

// unsupportedAWSBoolFlags registers boolean aws flags that are rejected with
// an explanation when set.
func unsupportedAWSBoolFlags(cmd *cobra.Command, flags map[string]string) {
	for name := range flags {
		cmd.Flags().Bool(name, false, "not supported on Latitude object storage")
		_ = cmd.Flags().MarkHidden(name)
	}
	pre := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		for name, why := range flags {
			if c.Flags().Changed(name) {
				return printErr(exitcode.Errorf(exitcode.Usage, "--%s is not supported on Latitude object storage: %s", name, why))
			}
		}
		if pre != nil {
			return pre(c, args)
		}
		return nil
	}
}

// joinNonEmpty joins the non-empty strings with sep.
func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// rejectRegionFlag registers --region on object commands (ls, cp, mv, sync,
// rm, stat, presign) so aws users get a directed explanation instead of an
// "unknown flag" error: on Latitude --region is a site (mb, access-keys) and
// the SigV4 signing region comes from the bucket's endpoint.
func rejectRegionFlag(cmd *cobra.Command) {
	unsupportedAWSFlags(cmd, map[string]string{
		"region": "on object commands the signing region is derived from the bucket's endpoint. To disambiguate a repeated bucket name by location use --site; to override the SigV4 signing region use --signing-region <region>",
	})
}

// outputExplicit reports whether the user asked for a structured format on
// the command line (-o/--output or --json), as opposed to LSH_OUTPUT or the
// config file. Secrets are only embedded in structured output on explicit
// request.
func outputExplicit(cmd *cobra.Command) bool {
	if viper.GetBool("output_explicit") {
		return true
	}
	if cmd == nil {
		return false
	}
	changed := cmd.Flags().Changed("json")
	if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil && f.Changed {
		changed = true
	}
	return changed
}

// awsRegionPattern matches AWS-style region names (us-east-1, sa-east-1…),
// which users coming from aws type by reflex where a Latitude site is expected.
var awsRegionPattern = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-[0-9]+$`)

// looksLikeAWSRegion reports whether s is an AWS region name.
func looksLikeAWSRegion(s string) bool {
	return awsRegionPattern.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

// emptyCell is the placeholder for empty table cells across the group.
const emptyCell = "-"

// orEmptyCell returns s, or the shared placeholder when empty.
func orEmptyCell(s string) string {
	if strings.TrimSpace(s) == "" {
		return emptyCell
	}
	return s
}

// saveNewKey stores a freshly created access key in the active profile under
// name, appending -2, -3… when that name already holds a different key so a
// saved key is never overwritten silently. It returns the name actually used
// and the profile name. Every creation path (access-keys create, configure,
// mb) goes through it so the clash policy is the same everywhere; rotate
// replaces in place with objectstorage.SaveKey instead.
func saveNewKey(cmd *cobra.Command, name string, k config.StoredAccessKey) (string, string, error) {
	_, profileName, profile, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return "", "", err
	}
	existing := profile.ObjectStorageKeys()
	final := name
	for i := 2; ; i++ {
		cur, taken := existing[final]
		if !taken || cur.AccessKeyID == k.AccessKeyID {
			break
		}
		final = fmt.Sprintf("%s-%d", name, i)
	}
	if _, err := objectstorage.SaveKey(profileFlag(cmd), final, k); err != nil {
		return final, profileName, err
	}
	return final, profileName, nil
}
