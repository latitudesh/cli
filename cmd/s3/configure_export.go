package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	cobra "github.com/spf13/cobra"
)

// Flag names of `lsh s3 configure export`.
const (
	flagExportFormat      = "format"
	flagExportProfileName = "profile-name"
	flagExportNoSecret    = "no-secret"
)

// exportFormats lists the supported --format values in help order.
var exportFormats = []string{"env", "aws", "rclone", "mc", "s3cmd", "process"}

// exportSecretPlaceholder replaces the secret with --no-secret.
const exportSecretPlaceholder = "<secret>"

// newConfigureExportCmd builds `lsh s3 configure export`.
func newConfigureExportCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:   "export [s3://bucket]",
		Short: "Print a saved access key in the format of another S3 tool",
		Long: `Print the credential and connection settings for a bucket in the format of
another tool: shell variables (env), an aws CLI profile (aws), an rclone
remote (rclone), a MinIO client alias (mc), an s3cmd config (s3cmd) or the
JSON of an aws credential_process (process).

With a bucket argument the endpoint, signing region and backend bucket name
come from the Latitude API and the credential is selected like every other
command (LSH_S3_* environment, --access-key, or the least-privileged saved key
covering the bucket). Without a bucket, --access-key names the saved key and
--endpoint-url supplies the endpoint.

The output goes to stdout and contains the secret unless --no-secret is
given; -o/--output is ignored because the template is the output. The aws
format prints two blocks that belong to two different files (~/.aws/config and
~/.aws/credentials), each labelled with a comment, so it cannot be appended to
a single file in one redirect.`,
		Example: `  eval "$(lsh s3 configure export s3://backups --format env)"
  lsh s3 configure export s3://backups --format aws          # two blocks: one per file, see the comments
  lsh s3 configure export s3://backups --format rclone --profile-name latitude >> ~/.config/rclone/rclone.conf
  lsh s3 configure export s3://backups --format mc | sh
  lsh s3 configure export --access-key ci-deploy --endpoint-url https://s3.us-central-1.storage.sh --format process
  lsh s3 configure export s3://backups --format s3cmd --no-secret`,
		Args: cobra.MaximumNArgs(1),
		RunE: runConfigureExport,
	})
	addProjectFlag(cmd, true, "project ID or slug to disambiguate the bucket name")
	cmd.Flags().String(flagExportFormat, "", "output format: "+strings.Join(exportFormats, " | "))
	cmd.Flags().String(flagExportProfileName, "", "profile/remote/alias name in the output (default: lsh-<bucket> or lsh-<key name>)")
	cmd.Flags().Bool(flagExportNoSecret, false, "replace the secret with a "+exportSecretPlaceholder+" placeholder")
	return cmd
}

// exportTarget is everything a template needs.
type exportTarget struct {
	ProfileName   string
	Endpoint      string // scheme://host[:port]
	Host          string // host[:port] without scheme
	Secure        bool
	SigningRegion string
	BucketName    string // backend bucket name; empty when unknown
	StorageClass  string // standard | high_performance | ""
	AccessKeyID   string
	Secret        string
}

// newExportTarget derives Host/Secure from the endpoint.
func newExportTarget(profileName, endpoint, region, bucketName, class, accessKeyID, secret string) (exportTarget, error) {
	host, secure, err := objectstorage.EndpointHost(endpoint)
	if err != nil {
		return exportTarget{}, exitcode.New(exitcode.Usage, err)
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	if region == "" {
		region = objectstorage.SigningRegion(endpoint)
	}
	return exportTarget{
		ProfileName:   profileName,
		Endpoint:      strings.TrimRight(endpoint, "/"),
		Host:          host,
		Secure:        secure,
		SigningRegion: region,
		BucketName:    bucketName,
		StorageClass:  class,
		AccessKeyID:   accessKeyID,
		Secret:        secret,
	}, nil
}

// bucketOrPlaceholder returns the backend bucket name or "<bucket>" for the
// example commands when no bucket is known.
func (t exportTarget) bucketOrPlaceholder() string {
	if t.BucketName == "" {
		return "<bucket>"
	}
	return t.BucketName
}

// header is the comment every text template starts with.
func (t exportTarget) header() string {
	if t.BucketName == "" {
		return ""
	}
	return "# bucket name on the endpoint: " + t.BucketName + "\n"
}

// rcloneProvider maps the storage class to rclone's provider setting: the
// standard tier is Wasabi-backed, high_performance (VAST) is generic S3.
func (t exportTarget) rcloneProvider() string {
	switch t.StorageClass {
	case objectstorage.ClassStandard:
		return "Wasabi"
	case objectstorage.ClassHighPerformance:
		return "Other"
	}
	// Infer from the endpoint shape when the class is unknown.
	if strings.HasPrefix(strings.ToLower(t.Host), "s3.") {
		return "Wasabi"
	}
	return "Other"
}

// renderExport dispatches to the template for format.
func renderExport(format string, t exportTarget) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "env":
		return exportEnv(t), nil
	case "aws":
		return exportAWS(t), nil
	case "rclone":
		return exportRclone(t), nil
	case "mc":
		return exportMc(t), nil
	case "s3cmd":
		return exportS3cmd(t), nil
	case "process":
		return exportProcess(t), nil
	}
	return "", objectstorage.ErrUsagef("--format must be one of %s (got %q)", strings.Join(exportFormats, ", "), format)
}

// cfgShellValue quotes v for a POSIX shell when it contains characters outside
// the safe set (aws export-credentials prints plain values; secrets are
// base64-like so they normally pass through untouched).
func cfgShellValue(v string) string {
	safe := true
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("_./:+=@-", r):
		default:
			safe = false
		}
	}
	if safe && v != "" {
		return v
	}
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// exportEnv renders shell exports for the aws CLI/SDKs and for lsh itself.
func exportEnv(t exportTarget) string {
	var b strings.Builder
	b.WriteString(t.header())
	fmt.Fprintf(&b, "export AWS_ACCESS_KEY_ID=%s\n", cfgShellValue(t.AccessKeyID))
	fmt.Fprintf(&b, "export AWS_SECRET_ACCESS_KEY=%s\n", cfgShellValue(t.Secret))
	fmt.Fprintf(&b, "export AWS_ENDPOINT_URL_S3=%s\n", cfgShellValue(t.Endpoint))
	fmt.Fprintf(&b, "export AWS_REGION=%s\n", cfgShellValue(t.SigningRegion))
	b.WriteString("export AWS_REQUEST_CHECKSUM_CALCULATION=when_required\n")
	b.WriteString("export AWS_RESPONSE_CHECKSUM_VALIDATION=when_required\n")
	fmt.Fprintf(&b, "export %s=%s\n", objectstorage.EnvAccessKeyID, cfgShellValue(t.AccessKeyID))
	fmt.Fprintf(&b, "export %s=%s\n", objectstorage.EnvSecretAccessKey, cfgShellValue(t.Secret))
	fmt.Fprintf(&b, "export %s=%s\n", objectstorage.EnvEndpointURL, cfgShellValue(t.Endpoint))
	fmt.Fprintf(&b, "export %s=%s\n", objectstorage.EnvSigningRegion, cfgShellValue(t.SigningRegion))
	fmt.Fprintf(&b, "# aws s3 ls s3://%s/\n", t.bucketOrPlaceholder())
	return b.String()
}

// exportAWS renders an aws CLI profile: a config block and a credentials
// block, each commented with the file it belongs to.
func exportAWS(t exportTarget) string {
	var b strings.Builder
	b.WriteString(t.header())
	b.WriteString("# ~/.aws/config\n")
	fmt.Fprintf(&b, "[profile %s]\n", t.ProfileName)
	fmt.Fprintf(&b, "region = %s\n", t.SigningRegion)
	fmt.Fprintf(&b, "endpoint_url = %s\n", t.Endpoint)
	b.WriteString("s3 =\n")
	b.WriteString("    addressing_style = path\n")
	b.WriteString("request_checksum_calculation = when_required\n")
	b.WriteString("response_checksum_validation = when_required\n")
	b.WriteString("\n")
	b.WriteString("# ~/.aws/credentials\n")
	fmt.Fprintf(&b, "[%s]\n", t.ProfileName)
	fmt.Fprintf(&b, "aws_access_key_id = %s\n", t.AccessKeyID)
	fmt.Fprintf(&b, "aws_secret_access_key = %s\n", t.Secret)
	b.WriteString("\n")
	fmt.Fprintf(&b, "# aws --profile %s s3 ls s3://%s/\n", t.ProfileName, t.bucketOrPlaceholder())
	return b.String()
}

// exportRclone renders an rclone remote section.
func exportRclone(t exportTarget) string {
	var b strings.Builder
	b.WriteString(t.header())
	fmt.Fprintf(&b, "[%s]\n", t.ProfileName)
	b.WriteString("type = s3\n")
	fmt.Fprintf(&b, "provider = %s\n", t.rcloneProvider())
	b.WriteString("env_auth = false\n")
	fmt.Fprintf(&b, "access_key_id = %s\n", t.AccessKeyID)
	fmt.Fprintf(&b, "secret_access_key = %s\n", t.Secret)
	fmt.Fprintf(&b, "endpoint = %s\n", t.Host)
	fmt.Fprintf(&b, "region = %s\n", t.SigningRegion)
	b.WriteString("force_path_style = true\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "# rclone ls %s:%s\n", t.ProfileName, t.bucketOrPlaceholder())
	return b.String()
}

// exportMc renders the MinIO client alias command.
func exportMc(t exportTarget) string {
	var b strings.Builder
	b.WriteString(t.header())
	fmt.Fprintf(&b, "mc alias set %s %s %s %s --api S3v4 --path on\n", cfgShellValue(t.ProfileName), cfgShellValue(t.Endpoint), cfgShellValue(t.AccessKeyID), cfgShellValue(t.Secret))
	fmt.Fprintf(&b, "# mc ls %s/%s\n", t.ProfileName, t.bucketOrPlaceholder())
	return b.String()
}

// exportS3cmd renders an s3cmd configuration.
func exportS3cmd(t exportTarget) string {
	useHTTPS := "True"
	if !t.Secure {
		useHTTPS = "False"
	}
	var b strings.Builder
	b.WriteString(t.header())
	b.WriteString("[default]\n")
	fmt.Fprintf(&b, "access_key = %s\n", t.AccessKeyID)
	fmt.Fprintf(&b, "secret_key = %s\n", t.Secret)
	fmt.Fprintf(&b, "host_base = %s\n", t.Host)
	fmt.Fprintf(&b, "host_bucket = %s\n", t.Host)
	fmt.Fprintf(&b, "bucket_location = %s\n", t.SigningRegion)
	fmt.Fprintf(&b, "use_https = %s\n", useHTTPS)
	b.WriteString("signature_v2 = False\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "# s3cmd ls s3://%s\n", t.bucketOrPlaceholder())
	return b.String()
}

// exportProcess renders the JSON an aws credential_process must print.
func exportProcess(t exportTarget) string {
	payload := struct {
		Version         int    `json:"Version"`
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
	}{Version: 1, AccessKeyID: t.AccessKeyID, SecretAccessKey: t.Secret}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // keep the <secret> placeholder readable
	if err := enc.Encode(payload); err != nil {
		return ""
	}
	return b.String() // Encode appends the trailing newline
}

// exportProfileName picks the default profile/remote name.
func exportProfileName(explicit string, b *objectstorage.Bucket, cred objectstorage.Credential) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if b != nil && b.Name != "" {
		return "lsh-" + b.Name
	}
	if cred.Name != "" {
		return "lsh-" + cred.Name
	}
	if cred.AccessKeyID != "" {
		return "lsh-" + strings.ToLower(cred.AccessKeyID)
	}
	return "lsh"
}

func runConfigureExport(cmd *cobra.Command, args []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	format, _ := cmd.Flags().GetString(flagExportFormat)
	if strings.TrimSpace(format) == "" {
		return printErr(objectstorage.ErrUsagef("--format is required: one of %s", strings.Join(exportFormats, ", ")))
	}
	if _, err := renderExport(format, exportTarget{}); err != nil {
		return printErr(err)
	}
	profileName, _ := cmd.Flags().GetString(flagExportProfileName)
	noSecret, _ := cmd.Flags().GetBool(flagExportNoSecret)
	accessKey, _ := cmd.Flags().GetString(flagAccessKey)

	var (
		bucket *objectstorage.Bucket
		cred   objectstorage.Credential
	)
	if len(args) == 1 {
		ref, err := objectstorage.ParseBucketOnly(args[0])
		if err != nil {
			return printErr(err)
		}
		bucket, err = resolveBucket(ctx, cmd, ref.Bucket)
		if err != nil {
			return printErr(err)
		}
		if err := bucket.Validate(); err != nil {
			return printErr(err)
		}
		cred, err = objectstorage.ResolveCredential(bucket, objectstorage.CredentialOptions{
			AccessKeyName:   accessKey,
			ProfileOverride: profileFlag(cmd),
		})
		if err != nil {
			return printErr(err)
		}
	} else {
		endpoint := endpointOverride(cmd)
		if endpoint == "" {
			return printErr(objectstorage.ErrUsagef("pass s3://<bucket>, or --access-key <name> together with --endpoint-url <url> (or %s)", objectstorage.EnvEndpointURL))
		}
		var err error
		cred, err = objectstorage.ResolveCredential(nil, objectstorage.CredentialOptions{
			AccessKeyName:   accessKey,
			ProfileOverride: profileFlag(cmd),
		})
		if err != nil {
			return printErr(err)
		}
		bucket = &objectstorage.Bucket{
			Endpoint:         endpoint,
			SigningRegion:    signingRegionOverride(cmd),
			StorageClass:     cred.Key.StorageClass,
			EndpointOverride: true,
		}
	}

	secret := cred.Secret()
	if noSecret {
		secret = exportSecretPlaceholder
	}
	target, err := newExportTarget(
		exportProfileName(profileName, bucket, cred),
		bucket.Endpoint, bucket.SigningRegion, bucket.BucketName, bucket.StorageClass,
		cred.AccessKeyID, secret,
	)
	if err != nil {
		return printErr(err)
	}
	out, err := renderExport(format, target)
	if err != nil {
		return printErr(err)
	}
	fmt.Print(out)
	if !noSecret {
		objectstorage.Warnf("output contains a secret; do not commit it")
	}
	return nil
}
