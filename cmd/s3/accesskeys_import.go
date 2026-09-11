package s3

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysImportCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:   "import --name <name> --access-key-id <id>",
		Short: "Save an existing access key in this profile (secret from stdin or a prompt)",
		Long: `Save an access key that already exists (created in the dashboard, by another
tool or on another machine) in the active profile.

The secret is never passed as a flag: it is read from a prompt without echo on
a terminal, or from a single line on stdin otherwise, so
'echo "$SECRET" | lsh s3 access-keys import ...' works in scripts.

When --project is known the key is matched against the API to record its scope
(class, site, buckets); otherwise --storage-class/--region/--bucket/--all-buckets
describe it. Keys with unknown scope are never selected automatically: they are
only used with --access-key <name>.`,
		Example: `  lsh s3 access-keys import --name legacy --access-key-id AKIA... --project my-project
  echo "$SECRET" | lsh s3 access-keys import --name ci --access-key-id AKIA... --bucket backups=rw
  lsh s3 access-keys import --name ops --access-key-id AKIA... --all-buckets --storage-class standard --project my-project`,
		Args: cobra.NoArgs,
		RunE: runAccessKeysImport,
	})
	addProjectFlag(cmd, true, "project of the key (ID or slug); lets the CLI look its scope up on the API")
	cmd.Flags().String("name", "", "name to save the key under (required)")
	cmd.Flags().String("access-key-id", "", "the access key ID (required)")
	cmd.Flags().StringP("storage-class", "c", "", "storage class of the key: standard or high_performance")
	cmd.Flags().String("region", "", "site of the key (e.g. TYO4) for high_performance keys")
	cmd.Flags().StringArray("bucket", nil, bucketSpecUsage)
	cmd.Flags().Bool("all-buckets", false, "the key covers every bucket of the project in its storage class (fullaccess)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("access-key-id")
	return cmd
}

// importScope resolves the metadata of an imported key: first from the API
// (when the project is known and the ID is listed), then from the flags.
func importScope(ctx context.Context, r *objectstorage.Resolver, accessKeyID string, o createOptions) (config.StoredAccessKey, error) {
	k := config.StoredAccessKey{
		AccessKeyID:  accessKeyID,
		StorageClass: o.StorageClass,
		Site:         o.Site,
		ProjectID:    o.Project,
		Scope:        config.ScopeUnknown,
		Source:       config.KeySourceImport,
		CreatedAt:    time.Now().UTC(),
	}

	if o.Project != "" && r.API != nil {
		scopes, err := objectstorage.RawAccessKeyScopes(ctx, o.Project)
		if err != nil {
			lsh.LogDebugf("[s3] could not look the key up on the API: %v", err)
		} else if s, ok := scopes[accessKeyID]; ok {
			if k.StorageClass == "" {
				k.StorageClass = s.StorageClass
			}
			if k.Site == "" {
				k.Site = strings.ToUpper(s.Site)
			}
			k.Username = s.Username
			switch s.Access {
			case config.ScopeFullAccess:
				k.Scope = config.ScopeFullAccess
			case config.PermissionRW, config.PermissionReadOnly:
				k.Scope = config.ScopeLimitedAccess
				k.Buckets = map[string]string{}
				scoped := *r
				scoped.Project = o.Project
				list, err := scoped.ListBuckets(ctx)
				if err != nil {
					return k, err
				}
				for _, d := range list {
					b := objectstorage.BucketFromData(d)
					for _, name := range s.Buckets {
						if b.BucketName == name || b.Name == name {
							k.Buckets[b.ID] = s.Access
							if k.ProjectID == "" || !strings.HasPrefix(k.ProjectID, "proj_") {
								k.ProjectID = b.ProjectID
							}
						}
					}
				}
			}
			if k.Scope != config.ScopeUnknown {
				return k, nil
			}
		}
	}

	switch {
	case o.AllBuckets && len(o.BucketSpecs) > 0:
		return k, exitcode.Errorf(exitcode.Usage, "--all-buckets and --bucket are mutually exclusive")
	case o.AllBuckets:
		k.Scope = config.ScopeFullAccess
		if k.StorageClass == "" {
			return k, exitcode.Errorf(exitcode.Usage, "--storage-class standard|high_performance is required with --all-buckets")
		}
		if k.StorageClass == objectstorage.ClassHighPerformance && k.Site == "" {
			return k, exitcode.Errorf(exitcode.Usage, "--region <site> is required for a high_performance key (e.g. TYO4)")
		}
	case len(o.BucketSpecs) > 0:
		var scoped []scopedBucket
		for _, spec := range o.BucketSpecs {
			b, err := r.Resolve(ctx, spec.Token)
			if err != nil {
				return k, err
			}
			if b.EndpointOverride {
				return k, exitcode.Errorf(exitcode.Usage, "access keys are managed through the Latitude API; unset --endpoint-url")
			}
			if err := r.FillSite(ctx, b); err != nil {
				lsh.LogDebugf("[s3] site lookup failed for %s: %v", b.ID, err)
			}
			scoped = append(scoped, scopedBucket{Bucket: b, Permission: spec.Permission})
		}
		if err := validateSameGroup(scoped); err != nil {
			return k, err
		}
		first := scoped[0].Bucket
		k.Scope = config.ScopeLimitedAccess
		k.Buckets = map[string]string{}
		for _, sb := range scoped {
			k.Buckets[sb.Bucket.ID] = sb.Permission
		}
		if k.StorageClass == "" {
			k.StorageClass = first.StorageClass
		}
		if k.Site == "" && first.StorageClass == objectstorage.ClassHighPerformance {
			k.Site = strings.ToUpper(first.Site)
		}
		if k.ProjectID == "" || !strings.HasPrefix(k.ProjectID, "proj_") {
			k.ProjectID = firstNonEmptyStr(first.ProjectID, k.ProjectID)
		}
	}
	return k, nil
}

func runAccessKeysImport(cmd *cobra.Command, _ []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	o, err := parseCreateOptions(cmd)
	if err != nil {
		return printErr(err)
	}
	accessKeyID, _ := cmd.Flags().GetString("access-key-id")
	accessKeyID = strings.TrimSpace(accessKeyID)
	if o.Name == "" || accessKeyID == "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "--name and --access-key-id are required"))
	}
	// Fail before reading the secret when there is no profile to save into.
	_, profileName, _, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return printErr(err)
	}

	r := newResolver(cmd)
	if r.EndpointURL != "" {
		r.API = nil
	}
	stored, err := importScope(ctx, r, accessKeyID, o)
	// keyMatchesBucket compares ProjectID with the bucket's proj_ ID, so a slug
	// passed on --project (accepted everywhere else) would leave the imported
	// key permanently unselectable. The --bucket paths already resolve it.
	if err == nil && stored.ProjectID != "" && !strings.HasPrefix(stored.ProjectID, "proj_") {
		stored.ProjectID = resolveProjectID(ctx, r, stored.ProjectID)
	}
	if err != nil {
		return printErr(err)
	}
	if stored.Scope == config.ScopeUnknown {
		objectstorage.Warnf("the scope of %s is unknown (not found on the API and no --bucket/--all-buckets given); the key is only used when selected explicitly with --access-key %s", accessKeyID, o.Name)
	}
	row := newSavedKeyRow(o.Name, stored, profileName)

	if dryRun() {
		if isHuman() {
			fmt.Printf("(dryrun) import: access key %s as %q in profile %s (class=%s site=%s scope=%s)\n", accessKeyID, o.Name, profileName, dash(stored.StorageClass), dash(stored.Site), stored.Scope)
			return nil
		}
		render([]renderer.ResponseData{row})
		return nil
	}

	secret, err := objectstorage.ReadSecret(cmd, fmt.Sprintf("Secret access key for %s: ", accessKeyID))
	if err != nil {
		return printErr(exitcode.Errorf(exitcode.Usage, "could not read the secret: %v", err))
	}
	if secret == "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "no secret provided; type it at the prompt or pipe it on stdin: echo \"$SECRET\" | lsh s3 access-keys import ..."))
	}
	stored.SecretAccessKey = secret
	// Same clash policy as create/configure/mb: a name holding another key
	// is never overwritten; the import is saved with a -2, -3… suffix.
	finalName, profileName, err := saveNewKey(cmd, o.Name, stored)
	if err != nil {
		return printErr(objectstorage.Humanize(err, nil, nil))
	}
	if finalName != o.Name {
		objectstorage.Hintf("a saved key named %q already exists; saved as %q instead", o.Name, finalName)
		row = newSavedKeyRow(finalName, stored, profileName)
	}
	if isHuman() {
		fmt.Printf("import: access key %s saved as %q in profile %s (scope %s)\n", accessKeyID, finalName, profileName, stored.Scope)
		return nil
	}
	render([]renderer.ResponseData{row})
	return nil
}
