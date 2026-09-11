package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysCreateCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "create",
		Aliases: []string{"new", "add"},
		Short:   "Create an access key for specific buckets or for all buckets of a class",
		Long: `Create an S3 access key through the Latitude API.

Scope is either specific buckets (--bucket, see its help for the grammar) or
every bucket of the project in one storage class (--all-buckets). All --bucket
values must share storage class, project and, for high_performance, site: one
key cannot span backends. Project, class and site are inferred from the buckets
when possible.

The secret is returned once by the API and cannot be retrieved again:
  - human output without --save prints it in clear, once;
  - --save stores the key in the active profile (under the key name, or the
    --save-as name) and does not print the secret (add --show-secret to print
    it too); a name already holding another key gets a -2, -3… suffix;
  - -o json|yaml|csv includes secret_access_key only when -o was passed
    explicitly or with --show-secret (never because of LSH_OUTPUT/config).

--like <saved-name> copies class, site, project and scope from a saved key so a
new bucket can be added to an existing scope (the API has no key update).`,
		Example: `  lsh s3 access-keys create --bucket backups=rw --bucket logs=readonly --name ci-deploy
  lsh s3 access-keys create --bucket backups --save
  lsh s3 access-keys create --all-buckets --storage-class standard --project my-project --name ops --save-as ops
  lsh s3 access-keys create --all-buckets --storage-class high_performance --region TYO4 --project my-project
  lsh s3 access-keys create --bucket backups=rw --name ci-deploy -o json | jq -r .secret_access_key
  lsh s3 access-keys create --like ci-deploy --bucket new-bucket=rw --name ci-deploy-v2`,
		Args: cobra.NoArgs,
		RunE: runAccessKeysCreate,
	})
	addProjectFlag(cmd, true, "project of the key (ID or slug); inferred from --bucket when omitted")
	cmd.Flags().StringArray("bucket", nil, bucketSpecUsage)
	cmd.Flags().Bool("all-buckets", false, "grant access to every bucket of the project in the storage class (fullaccess)")
	cmd.Flags().StringP("storage-class", "c", "", "storage class of the key: standard or high_performance (inferred from --bucket)")
	cmd.Flags().String("region", "", "site of the key (e.g. TYO4); required for high_performance when it cannot be inferred")
	cmd.Flags().String("name", "", "key name (default lsh-<user>-<bucket|site|class>); normalized server-side")
	cmd.Flags().Bool("save", false, "save the key in the active profile under the key name (the secret is then not printed)")
	cmd.Flags().String("save-as", "", "save the key in the active profile under this name (implies --save)")
	cmd.Flags().Bool("show-secret", false, "print the secret even when saving or when -o comes from the environment")
	cmd.Flags().String("like", "", "copy storage class, site, project and scope from this saved key")
	return cmd
}

// createOptions are the parsed flags of create (also used by rotate/mb).
type createOptions struct {
	BucketSpecs  []bucketSpec
	AllBuckets   bool
	StorageClass string
	Site         string
	Project      string
	Name         string
	Save         bool
	SaveName     string
	ShowSecret   bool
	Like         string
}

// createPlan is everything needed to call the API and print the result.
type createPlan struct {
	Request  accessKeyRequest
	Buckets  []scopedBucket // resolved --bucket values (empty for fullaccess)
	Endpoint string
	Signing  string
	// ProjectID is the bkt_ project ID to store with the key when known.
	ProjectID string
}

// parseCreateOptions reads the flags.
func parseCreateOptions(cmd *cobra.Command) (createOptions, error) {
	var o createOptions
	values, _ := cmd.Flags().GetStringArray("bucket")
	specs, err := parseBucketSpecs(values)
	if err != nil {
		return o, err
	}
	o.BucketSpecs = specs
	o.AllBuckets, _ = cmd.Flags().GetBool("all-buckets")
	if o.StorageClass, err = storageClassFlag(cmd); err != nil {
		return o, err
	}
	if o.Site, err = regionFlag(cmd); err != nil {
		return o, err
	}
	o.Project = projectFlag(cmd)
	o.Name, _ = cmd.Flags().GetString("name")
	o.Name = strings.TrimSpace(o.Name)
	if err := validateAccessKeyName(o.Name); err != nil {
		return o, err
	}
	// --save is a plain bool (so --save=false never saves); --save-as <name>
	// picks the saved name and implies --save.
	o.Save, _ = cmd.Flags().GetBool("save")
	o.SaveName, _ = cmd.Flags().GetString("save-as")
	o.SaveName = strings.TrimSpace(o.SaveName)
	if o.SaveName != "" {
		o.Save = true
	}
	o.ShowSecret, _ = cmd.Flags().GetBool("show-secret")
	o.Like, _ = cmd.Flags().GetString("like")
	return o, nil
}

// applyLike fills the options from a saved key template. Explicit flags win.
func applyLike(o *createOptions, like config.StoredAccessKey) error {
	if like.Scope == config.ScopeUnknown {
		return exitcode.Errorf(exitcode.Usage, "saved key %q has unknown scope and cannot be used as a template", o.Like)
	}
	if o.StorageClass == "" {
		o.StorageClass = like.StorageClass
	}
	if o.Site == "" {
		o.Site = strings.ToUpper(like.Site)
	}
	if o.Project == "" {
		o.Project = like.ProjectID
	}
	if !o.AllBuckets && len(o.BucketSpecs) == 0 && like.Scope == config.ScopeFullAccess {
		o.AllBuckets = true
	}
	if like.Scope == config.ScopeLimitedAccess && !o.AllBuckets {
		seen := map[string]bool{}
		for _, s := range o.BucketSpecs {
			seen[s.Token] = true
		}
		ids := make([]string, 0, len(like.Buckets))
		for id := range like.Buckets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if !seen[id] {
				o.BucketSpecs = append(o.BucketSpecs, bucketSpec{Token: id, Permission: like.Buckets[id]})
			}
		}
	}
	return nil
}

// buildCreatePlan resolves the buckets, validates that they belong to one
// backend and infers project, class and site.
func buildCreatePlan(ctx context.Context, r *objectstorage.Resolver, o createOptions) (*createPlan, error) {
	if o.AllBuckets && len(o.BucketSpecs) > 0 {
		return nil, exitcode.Errorf(exitcode.Usage, "--all-buckets and --bucket are mutually exclusive")
	}
	if !o.AllBuckets && len(o.BucketSpecs) == 0 {
		return nil, exitcode.Errorf(exitcode.Usage, "choose the scope: --bucket <name>[=rw|readonly] (repeatable) or --all-buckets --storage-class <class>")
	}
	plan := &createPlan{Request: accessKeyRequest{Project: o.Project, StorageClass: o.StorageClass, Site: o.Site, Name: o.Name}}

	if len(o.BucketSpecs) > 0 {
		for _, spec := range o.BucketSpecs {
			b, err := r.Resolve(ctx, spec.Token)
			if err != nil {
				return nil, err
			}
			if b.EndpointOverride {
				return nil, exitcode.Errorf(exitcode.Usage, "access keys are managed through the Latitude API; unset --endpoint-url")
			}
			if err := r.FillSite(ctx, b); err != nil {
				if b.StorageClass == objectstorage.ClassHighPerformance && o.Site == "" {
					return nil, err
				}
				lsh.LogDebugf("[s3] site lookup failed for %s: %v", b.ID, err)
			}
			plan.Buckets = append(plan.Buckets, scopedBucket{Bucket: b, Permission: spec.Permission})
		}
		if err := validateSameGroup(plan.Buckets); err != nil {
			return nil, err
		}
		first := plan.Buckets[0].Bucket
		plan.ProjectID = first.ProjectID
		if plan.Request.Project == "" {
			plan.Request.Project = first.ProjectRef()
		}
		if plan.Request.StorageClass == "" {
			plan.Request.StorageClass = first.StorageClass
		} else if plan.Request.StorageClass != first.StorageClass {
			return nil, exitcode.Errorf(exitcode.Usage, "--storage-class %s does not match the buckets (%s)", plan.Request.StorageClass, first.StorageClass)
		}
		if plan.Request.Site == "" {
			plan.Request.Site = strings.ToUpper(first.Site)
		}
		plan.Request.Scope = config.ScopeLimitedAccess
		plan.Request.Buckets = map[string]string{}
		for _, sb := range plan.Buckets {
			plan.Request.Buckets[sb.Bucket.ID] = sb.Permission
		}
		plan.Endpoint, plan.Signing = first.Endpoint, first.SigningRegion
	} else {
		plan.Request.Scope = config.ScopeFullAccess
		if plan.Request.Project == "" {
			return nil, exitcode.Errorf(exitcode.Usage, "--project is required with --all-buckets (the key covers every bucket of one project)")
		}
		if err := inferFromProject(ctx, r, plan); err != nil {
			return nil, err
		}
	}

	if plan.Request.Project == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "could not infer the project; pass --project <id|slug>")
	}
	if plan.Request.StorageClass == objectstorage.ClassHighPerformance && plan.Request.Site == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "--region <site> is required for high_performance keys and could not be inferred; pass the site slug of the VAST cluster (e.g. TYO4)")
	}
	if plan.Request.Site == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "--region <site> is required and could not be inferred from the buckets; pass a Latitude site slug (e.g. DAL)")
	}
	return plan, nil
}

// inferFromProject fills class, site, endpoint and project ID for a
// fullaccess key from the project's buckets.
func inferFromProject(ctx context.Context, r *objectstorage.Resolver, plan *createPlan) error {
	scoped := *r
	scoped.Project = plan.Request.Project
	list, err := scoped.ListBuckets(ctx)
	if err != nil {
		return err
	}
	var buckets []*objectstorage.Bucket
	classes := map[string]bool{}
	for _, d := range list {
		b := objectstorage.BucketFromData(d)
		if plan.ProjectID == "" {
			plan.ProjectID = b.ProjectID
		}
		classes[b.StorageClass] = true
		if plan.Request.StorageClass == "" || b.StorageClass == plan.Request.StorageClass {
			buckets = append(buckets, b)
		}
	}
	if plan.Request.StorageClass == "" {
		switch len(classes) {
		case 1:
			for c := range classes {
				plan.Request.StorageClass = c
			}
		case 0:
			return exitcode.Errorf(exitcode.Usage, "project %s has no buckets yet; pass --storage-class standard|high_performance (and --region <site>)", plan.Request.Project)
		default:
			return exitcode.Errorf(exitcode.Usage, "project %s has buckets of several storage classes; pass --storage-class standard or high_performance", plan.Request.Project)
		}
		buckets = buckets[:0]
		for _, d := range list {
			b := objectstorage.BucketFromData(d)
			if b.StorageClass == plan.Request.StorageClass {
				buckets = append(buckets, b)
			}
		}
	}
	// Sites come from the raw API (the SDK model drops them).
	if len(buckets) > 0 {
		sites, err := objectstorage.RawBucketSites(ctx, "")
		if err != nil {
			lsh.LogDebugf("[s3] site lookup failed: %v", err)
		}
		distinct := map[string]bool{}
		for _, b := range buckets {
			if b.Site == "" {
				b.Site = sites[b.ID]
			}
			if b.Site != "" {
				distinct[strings.ToUpper(b.Site)] = true
			}
		}
		if plan.Request.Site == "" {
			if plan.Request.StorageClass == objectstorage.ClassHighPerformance && len(distinct) > 1 {
				names := make([]string, 0, len(distinct))
				for s := range distinct {
					names = append(names, s)
				}
				sort.Strings(names)
				return exitcode.Errorf(exitcode.Usage, "project %s has high_performance buckets in several sites (%s); pass --region <site>", plan.Request.Project, strings.Join(names, ", "))
			}
			for _, b := range buckets {
				if b.Site != "" {
					plan.Request.Site = strings.ToUpper(b.Site)
					break
				}
			}
		}
		for _, b := range buckets {
			if plan.Request.StorageClass == objectstorage.ClassHighPerformance && !strings.EqualFold(b.Site, plan.Request.Site) {
				continue
			}
			if b.Endpoint != "" {
				plan.Endpoint, plan.Signing = b.Endpoint, b.SigningRegion
				break
			}
		}
	}
	return nil
}

// defaultCreateName generates a pet name for the plan's storage class, the
// same style the dashboard suggests (key-<adjective>-<noun>-<tier>).
func defaultCreateName(cmd *cobra.Command, plan *createPlan) string {
	_ = cmd
	return generateKeyName(plan.Request.StorageClass)
}

// bucketPermView is one bucket in the JSON output.
type bucketPermView struct {
	BucketName string `json:"bucket_name"`
	Name       string `json:"name,omitempty"`
	ID         string `json:"id,omitempty"`
	Permission string `json:"permission"`
}

// createdKeyView is the normalized JSON shape of a created key.
type createdKeyView struct {
	Name            string           `json:"name"`
	AccessKeyID     string           `json:"access_key_id"`
	SecretAccessKey string           `json:"secret_access_key,omitempty"`
	Username        string           `json:"username,omitempty"`
	Status          string           `json:"status,omitempty"`
	Scope           string           `json:"scope"`
	StorageClass    string           `json:"storage_class"`
	Site            string           `json:"site,omitempty"`
	Project         string           `json:"project"`
	Endpoint        string           `json:"endpoint,omitempty"`
	SigningRegion   string           `json:"signing_region,omitempty"`
	Buckets         []bucketPermView `json:"buckets"`
	SavedAs         string           `json:"saved_as,omitempty"`
	Profile         string           `json:"profile,omitempty"`
	DryRun          bool             `json:"dry_run,omitempty"`
}

func newCreatedKeyView(k *createdAccessKey, plan *createPlan, showSecret bool) createdKeyView {
	v := createdKeyView{
		Name: k.Name, AccessKeyID: k.AccessKeyID, Username: k.Username, Status: k.Status, Scope: k.Scope,
		StorageClass: k.StorageClass, Site: k.Site, Project: k.Project, Endpoint: k.Endpoint, SigningRegion: k.SigningRegion,
		Buckets: []bucketPermView{},
	}
	if showSecret {
		v.SecretAccessKey = k.SecretAccessKey
	}
	for _, sb := range plan.Buckets {
		v.Buckets = append(v.Buckets, bucketPermView{BucketName: sb.Bucket.BucketName, Name: sb.Bucket.Name, ID: sb.Bucket.ID, Permission: sb.Permission})
	}
	return v
}

func (v createdKeyView) TableRow() table.Row {
	status := "created"
	if v.DryRun {
		status = "dryrun"
	}
	return table.Row{
		"name":          {Label: "Name", Value: v.Name},
		"access_key_id": {Label: "Access Key ID", Value: v.AccessKeyID},
		"scope":         {Label: "Scope", Value: v.Scope},
		"storage_class": {Label: "Class", Value: v.StorageClass},
		"region":        {Label: "Site", Value: dash(v.Site)},
		"project":       {Label: "Project", Value: v.Project},
		"status":        {Label: "Status", Value: status},
	}
}

// bucketsLine renders "backups-7f3a (rw), logs-91aa (readonly)" or the
// fullaccess description.
func (v createdKeyView) bucketsLine() string {
	if v.Scope == config.ScopeFullAccess {
		s := fmt.Sprintf("all %s buckets of project %s", v.StorageClass, v.Project)
		if v.Site != "" {
			s += " in " + v.Site
		}
		return s
	}
	parts := make([]string, 0, len(v.Buckets))
	for _, b := range v.Buckets {
		parts = append(parts, fmt.Sprintf("%s (%s)", firstNonEmptyStr(b.BucketName, b.Name, b.ID), b.Permission))
	}
	return strings.Join(parts, ", ")
}

// printCreatedHuman prints the J3 block. The secret line is only present
// when showSecret is true; it is never masked.
func printCreatedHuman(w io.Writer, v createdKeyView, showSecret bool) {
	if v.DryRun {
		fmt.Fprintf(w, "(dryrun) create access key %q (no API call made):\n", v.Name)
	} else if showSecret {
		fmt.Fprintf(w, "Access key %q created. The secret is shown once and cannot be retrieved again:\n", v.Name)
	} else {
		fmt.Fprintf(w, "Access key %q created.\n", v.Name)
	}
	if v.AccessKeyID != "" {
		fmt.Fprintf(w, "  Access Key ID:      %s\n", v.AccessKeyID)
	}
	if showSecret && v.SecretAccessKey != "" {
		fmt.Fprintf(w, "  Secret Access Key:  %s\n", v.SecretAccessKey)
	}
	if v.Endpoint != "" {
		fmt.Fprintf(w, "  Endpoint:           %s\n", v.Endpoint)
	} else {
		fmt.Fprintf(w, "  Endpoint:           (see 'lsh s3 get s3://<bucket>' for the bucket endpoint)\n")
	}
	if v.SigningRegion != "" {
		fmt.Fprintf(w, "  Signing region:     %s\n", v.SigningRegion)
	}
	fmt.Fprintf(w, "  Class:              %s\n", v.StorageClass)
	if v.Site != "" {
		fmt.Fprintf(w, "  Site:               %s\n", v.Site)
	}
	fmt.Fprintf(w, "  Project:            %s\n", v.Project)
	fmt.Fprintf(w, "  Scope:              %s\n", v.Scope)
	fmt.Fprintf(w, "  Buckets:            %s\n", v.bucketsLine())
	if v.SavedAs != "" {
		fmt.Fprintf(w, "  Saved as:           %q in profile %s\n", v.SavedAs, v.Profile)
	}
}

func runAccessKeysCreate(cmd *cobra.Command, _ []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	o, err := parseCreateOptions(cmd)
	if err != nil {
		return printErr(err)
	}
	if o.Like != "" {
		_, profileName, p, err := objectstorage.ActiveProfile(profileFlag(cmd))
		if err != nil {
			return printErr(err)
		}
		like, ok := p.ObjectStorageKeys()[o.Like]
		if !ok {
			return printErr(exitcode.Errorf(exitcode.NotFound, "no saved access key named %q in profile %s; run 'lsh s3 access-keys list --saved'", o.Like, profileName))
		}
		if err := applyLike(&o, like); err != nil {
			return printErr(err)
		}
	}
	// Fail before creating anything if the key could not be saved afterwards:
	// the secret would be lost.
	profileName := ""
	if o.Save {
		_, name, _, err := objectstorage.ActiveProfile(profileFlag(cmd))
		if err != nil {
			return printErr(exitcode.Errorf(exitcode.Of(err), "--save needs an active profile: %v", err))
		}
		profileName = name
	}

	r := newResolver(cmd)
	if r.EndpointURL != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "access keys are managed through the Latitude API; unset --endpoint-url"))
	}
	plan, err := buildCreatePlan(ctx, r, o)
	if err != nil {
		return printErr(err)
	}
	if plan.Request.Name == "" {
		plan.Request.Name = defaultCreateName(cmd, plan)
	}
	saveName := o.SaveName
	if o.Save && saveName == "" {
		saveName = plan.Request.Name
	}

	decision := decideSecretDisplay(isHuman(), o.Save, o.ShowSecret, outputExplicit(cmd))

	if dryRun() {
		k := &createdAccessKey{
			Name: plan.Request.Name, StorageClass: plan.Request.StorageClass, Site: plan.Request.Site,
			Project: plan.Request.Project, Scope: plan.Request.Scope, Endpoint: plan.Endpoint, SigningRegion: plan.Signing,
		}
		v := newCreatedKeyView(k, plan, false)
		v.DryRun = true
		if o.Save {
			v.SavedAs, v.Profile = saveName, profileName
		}
		if isHuman() {
			printCreatedHuman(os.Stdout, v, false)
		} else {
			render([]renderer.ResponseData{v})
		}
		return nil
	}

	// The API name is generated whenever --name was not given; --save-as only
	// names the local alias, so it must not disable the collision re-roll.
	created, err := createAccessKeyRetrying(ctx, cmd, plan.Request, o.Name == "")
	if err != nil {
		return printErr(err)
	}
	// A re-roll changed the API name: the alias has to follow it, or it would
	// keep pointing at the name of another live key.
	if o.SaveName == "" && created.Name != "" && created.Name != saveName {
		saveName = created.Name
	}
	created.Endpoint, created.SigningRegion = plan.Endpoint, plan.Signing
	if len(plan.Buckets) > 0 {
		created.Buckets = map[string]string{}
		for _, sb := range plan.Buckets {
			created.Buckets[sb.Bucket.Name] = sb.Permission
		}
	}
	v := newCreatedKeyView(created, plan, decision.Show)

	if o.Save {
		stored := created.stored(plan.ProjectID, plan.Request.Buckets, config.KeySourceCreate)
		// saveNewKey never overwrites another key: a taken name gets -2, -3…
		finalName, savedProfile, err := saveNewKey(cmd, saveName, stored)
		if err != nil {
			// "it was saved in the profile" would be a lie now.
			decision.Warning = ""
			// Removing the key is preferred over printing its secret; the
			// helper only prints when it cannot remove it (or --show-secret
			// asked for it), and always on stderr, never on stdout.
			reportUnsavedKey(ctx, os.Stderr, plan.Request, created, plan, saveName, o.ShowSecret, err)
			// The command's contract with --save is create *and* persist, and
			// the key may have been removed again: exiting 0 here would tell
			// automation the credential is in the profile.
			return printErr(exitcode.Errorf(exitcode.Generic, "access key %q was created but not saved in the profile", saveName))
		} else {
			v.SavedAs, v.Profile = finalName, savedProfile
			if finalName != saveName {
				objectstorage.Hintf("a saved key named %q already exists; saved as %q instead", saveName, finalName)
			}
			objectstorage.Hintf("Saved as %q in profile %s. The CLI uses it automatically for the buckets it covers.", finalName, savedProfile)
		}
	}
	if decision.Warning != "" {
		objectstorage.Warnf("%s", decision.Warning)
	}
	if isHuman() {
		printCreatedHuman(os.Stdout, v, decision.Show)
		return nil
	}
	render([]renderer.ResponseData{v})
	return nil
}
