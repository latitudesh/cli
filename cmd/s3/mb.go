package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

// Flag names of mb.
const (
	flagMbName            = "name"
	flagMbRegion          = "region"
	flagMbStorageClass    = "storage-class"
	flagMbVersioning      = "versioning"
	flagMbLocking         = "locking"
	flagMbRetentionMode   = "retention-mode"
	flagMbRetentionDays   = "retention-days"
	flagMbCreateAccessKey = "create-access-key"
	flagMbNoAccessKey     = "no-access-key"
)

// mbOptions are the validated inputs of `mb`.
type mbOptions struct {
	Name         string
	Project      string
	Region       string
	StorageClass string
	// Versioning is the --versioning value; VersioningSet records whether the
	// flag was given explicitly (so --locking --versioning=false is caught).
	Versioning    bool
	VersioningSet bool
	Locking       bool
	RetentionMode string
	RetentionDays int64
}

// NewMbCmd builds `lsh s3 mb s3://bucket`.
func NewMbCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "mb s3://bucket",
		Aliases: []string{"create"},
		Short:   "Create a bucket",
		Long: `Create an object storage bucket through the Latitude API.

The bucket is created in a Latitude site (--region DAL, NYC, TYO4…). The
storage class picks the backend tier: standard (default) or
high_performance (select sites only). Versioning, object lock and the default
retention cannot be changed after creation.

In an interactive session missing --project and --region are asked for, and
right after the bucket is created the CLI offers to create and save an S3
access key so cp/ls/rm work immediately; use --create-access-key or
--no-access-key in scripts.`,
		Example: `  lsh s3 mb s3://backups --region DAL --project my-project
  lsh s3 mb s3://fast-cache --region TYO4 --storage-class high_performance
  lsh s3 mb s3://audit --region NYC --locking --retention-mode COMPLIANCE --retention-days 30
  lsh s3 mb s3://ci-artifacts --region DAL --create-access-key
  lsh s3 mb s3://backups --region DAL -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runMb,
	})
	// Registered as required so the root pre-run opens the shared project
	// picker (or fails with exit 2 in a non-interactive session), like every
	// other project-scoped command.
	addProjectFlag(cmd, false, "project ID or slug that owns the bucket (required; a picker opens when omitted in a terminal)")
	cmd.Flags().String(flagMbName, "", "bucket name (legacy alternative to the positional s3://bucket)")
	_ = cmd.Flags().MarkHidden(flagMbName)
	cmd.Flags().String(flagMbRegion, "", "Latitude site slug where the bucket lives (e.g. DAL, NYC, TYO4); see 'lsh regions list'")
	cmd.Flags().StringP(flagMbStorageClass, "c", objectstorage.ClassStandard, "storage class: standard or high_performance (aliases: std, hp)")
	cmd.Flags().Bool(flagMbVersioning, false, "enable object versioning")
	cmd.Flags().Bool(flagMbLocking, false, "enable object lock (WORM); implies --versioning and cannot be added later")
	cmd.Flags().String(flagMbRetentionMode, "", "default object lock retention mode: NONE, GOVERNANCE or COMPLIANCE (requires --locking)")
	cmd.Flags().Int64(flagMbRetentionDays, 0, "default object lock retention period in days (requires --locking)")
	cmd.Flags().Bool(flagMbCreateAccessKey, false, "create and save an access key for the new bucket's class without asking")
	cmd.Flags().Bool(flagMbNoAccessKey, false, "never offer to create an access key")
	unsupportedAWSFlags(cmd, map[string]string{
		"tags": "buckets have no tags; use the bucket name or project to organise them",
	})
	return cmd
}

func runMb(cmd *cobra.Command, args []string) error {
	o, err := mbOptionsFromCmd(cmd, args)
	if err != nil {
		return printErr(err)
	}
	if endpointOverride(cmd) != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "mb creates buckets through the Latitude API; unset --endpoint-url / %s", objectstorage.EnvEndpointURL))
	}
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	if o.Region == "" {
		region, err := pickRegion(ctx, cmd)
		if err != nil {
			return printErr(err)
		}
		o.Region = region
	}
	req, err := buildCreateBucketRequest(o)
	if err != nil {
		return printErr(err)
	}
	a := req.Data.Attributes

	if dryRun() {
		if isHuman() {
			fmt.Printf("(dryrun) make_bucket: s3://%s\n", o.Name)
			fmt.Println("  " + describeCreateRequest(a))
		} else {
			render([]renderer.ResponseData{mbPlan{Attributes: a, DryRun: true}})
		}
		return nil
	}

	if o.Locking {
		objectstorage.Warnf("object lock, versioning and the default retention cannot be changed after the bucket is created")
		if strings.EqualFold(o.RetentionMode, string(operations.RetentionModeCompliance)) && objectstorage.CanPrompt(cmd) {
			ok, err := objectstorage.Confirm(fmt.Sprintf("COMPLIANCE retention of %d days cannot be shortened or bypassed by anyone. Create bucket %s?", o.RetentionDays, o.Name))
			if err != nil {
				return printErr(objectstorage.Humanize(err, nil, nil))
			}
			if !ok {
				return printErr(exitcode.Errorf(exitcode.Refused, "cancelled"))
			}
		}
	}

	client := apiClient()
	resp, err := client.ObjectStorage.PostStorageBuckets(ctx, req, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return printErr(objectstorage.Humanize(err, nil, nil))
	}
	if resp == nil || resp.Object == nil || resp.Object.Data == nil {
		return printErr(exitcode.Errorf(exitcode.Generic, "the API returned no bucket for %s", o.Name))
	}
	b := objectstorage.BucketFromData(*resp.Object.Data)
	if b.Name == "" {
		b.Name = o.Name
	}
	if b.Site == "" {
		b.Site = strings.ToUpper(o.Region)
	}
	if b.StorageClass == "" {
		b.StorageClass = string(*a.StorageClass)
	}
	if b.ProjectID == "" && b.ProjectSlug == "" {
		b.ProjectSlug = o.Project
	}

	if isHuman() {
		fmt.Printf("make_bucket: s3://%s\n", b.Name)
		for _, line := range describeCreatedBucket(b) {
			fmt.Println("  " + line)
		}
	} else {
		render([]renderer.ResponseData{NewBucketRow(b)})
	}

	offerAccessKey(ctx, cmd, b, o)
	if isHuman() {
		objectstorage.Hintf("Try: lsh s3 cp ./file s3://%s/", b.Name)
	}
	return nil
}

// mbOptionsFromCmd reads the flags and positional argument.
func mbOptionsFromCmd(cmd *cobra.Command, args []string) (mbOptions, error) {
	f := cmd.Flags()
	o := mbOptions{Project: projectFlag(cmd)}
	legacy, _ := f.GetString(flagMbName)
	switch {
	case len(args) == 1 && legacy != "" && !sameBucketToken(args[0], legacy):
		return o, exitcode.Errorf(exitcode.Usage, "bucket given twice (%q and --name %q); pass only one", args[0], legacy)
	case len(args) == 1:
		ref, err := objectstorage.ParseBucketOnly(args[0])
		if err != nil {
			return o, err
		}
		o.Name = ref.Bucket
	case legacy != "":
		ref, err := objectstorage.ParseBucketOnly(legacy)
		if err != nil {
			return o, err
		}
		o.Name = ref.Bucket
	default:
		return o, exitcode.Errorf(exitcode.Usage, "missing bucket: usage 'lsh s3 mb s3://<bucket> --region <site>'")
	}
	o.Region, _ = f.GetString(flagMbRegion)
	o.StorageClass, _ = f.GetString(flagMbStorageClass)
	o.Versioning, _ = f.GetBool(flagMbVersioning)
	o.VersioningSet = f.Changed(flagMbVersioning)
	o.Locking, _ = f.GetBool(flagMbLocking)
	o.RetentionMode, _ = f.GetString(flagMbRetentionMode)
	o.RetentionDays, _ = f.GetInt64(flagMbRetentionDays)
	return o, nil
}

func sameBucketToken(a, b string) bool {
	ra, errA := objectstorage.ParseBucketOnly(a)
	rb, errB := objectstorage.ParseBucketOnly(b)
	return errA == nil && errB == nil && ra.Bucket == rb.Bucket
}

// buildCreateBucketRequest validates the options and builds the API request.
// It is pure so the rules can be unit-tested without cobra or the API.
func buildCreateBucketRequest(o mbOptions) (operations.PostStorageBucketsRequestBody, error) {
	var req operations.PostStorageBucketsRequestBody
	name := strings.TrimSpace(o.Name)
	if name == "" {
		return req, exitcode.Errorf(exitcode.Usage, "missing bucket name: usage 'lsh s3 mb s3://<bucket> --region <site>'")
	}
	if strings.ContainsAny(name, " /\\") {
		return req, exitcode.Errorf(exitcode.Usage, "invalid bucket name %q: it cannot contain spaces or slashes", name)
	}
	project := strings.TrimSpace(o.Project)
	if project == "" {
		return req, exitcode.Errorf(exitcode.Usage, "--project is required (or set LSH_PROJECT): the bucket must belong to a project")
	}
	region := strings.TrimSpace(o.Region)
	if region == "" {
		return req, exitcode.Errorf(exitcode.Usage, "--region <site> is required; regions are Latitude sites such as DAL, NYC or TYO4 (run 'lsh regions list')")
	}
	if looksLikeAWSRegion(region) {
		return req, exitcode.Errorf(exitcode.Usage, "%q is not a Latitude site; --region takes a site slug such as DAL, NYC or TYO4 (run 'lsh regions list')", region)
	}

	// The shared alias table (std, hp, high-performance…) applies here too.
	class, err := objectstorage.ParseStorageClass(o.StorageClass)
	if err != nil {
		return req, exitcode.Errorf(exitcode.Usage, "invalid --storage-class %q: use standard or high_performance", o.StorageClass)
	}
	if class == "" {
		class = objectstorage.ClassStandard
	}
	var storageClass operations.StorageClass
	switch class {
	case objectstorage.ClassHighPerformance:
		storageClass = operations.StorageClassHighPerformance
	default:
		storageClass = operations.StorageClassStandard
	}

	mode := strings.ToUpper(strings.TrimSpace(o.RetentionMode))
	var retentionMode operations.RetentionMode
	switch mode {
	case "", string(operations.RetentionModeNone):
		retentionMode = operations.RetentionModeNone
	case string(operations.RetentionModeGovernance):
		retentionMode = operations.RetentionModeGovernance
	case string(operations.RetentionModeCompliance):
		retentionMode = operations.RetentionModeCompliance
	default:
		return req, exitcode.Errorf(exitcode.Usage, "invalid --retention-mode %q: use NONE, GOVERNANCE or COMPLIANCE", o.RetentionMode)
	}
	if o.RetentionDays < 0 {
		return req, exitcode.Errorf(exitcode.Usage, "--retention-days must be a positive number of days")
	}
	hasRetention := retentionMode != operations.RetentionModeNone || o.RetentionDays > 0
	if hasRetention && !o.Locking {
		return req, exitcode.Errorf(exitcode.Usage, "--retention-mode and --retention-days require --locking (object lock must be enabled at creation)")
	}
	if retentionMode != operations.RetentionModeNone && o.RetentionDays == 0 {
		return req, exitcode.Errorf(exitcode.Usage, "--retention-mode %s requires --retention-days <N>", retentionMode)
	}
	if o.RetentionDays > 0 && retentionMode == operations.RetentionModeNone {
		return req, exitcode.Errorf(exitcode.Usage, "--retention-days requires --retention-mode GOVERNANCE or COMPLIANCE")
	}

	versioning := o.Versioning
	if o.Locking {
		if o.VersioningSet && !o.Versioning {
			return req, exitcode.Errorf(exitcode.Usage, "--locking requires versioning; drop --versioning=false")
		}
		versioning = true
	}

	attrs := operations.PostStorageBucketsAttributes{
		Project:       project,
		Name:          name,
		Region:        strings.ToUpper(region),
		StorageClass:  &storageClass,
		Versioning:    &versioning,
		Locking:       mbPtr(o.Locking),
		RetentionMode: &retentionMode,
	}
	if o.RetentionDays > 0 {
		attrs.RetentionPeriod = mbPtr(o.RetentionDays)
	}
	req.Data = operations.PostStorageBucketsData{
		Type:       operations.PostStorageBucketsTypeObjects,
		Attributes: attrs,
	}
	return req, nil
}

func mbPtr[T any](v T) *T { return &v }

// describeCreateRequest renders the request for dry-run output.
func describeCreateRequest(a operations.PostStorageBucketsAttributes) string {
	parts := []string{
		"project: " + a.Project,
		"region: " + a.Region,
		"class: " + string(*a.StorageClass),
		"versioning: " + yesNo(a.Versioning != nil && *a.Versioning),
	}
	if a.Locking != nil && *a.Locking {
		lock := "yes"
		if a.RetentionMode != nil && *a.RetentionMode != operations.RetentionModeNone {
			lock = string(*a.RetentionMode)
			if a.RetentionPeriod != nil {
				lock = fmt.Sprintf("%s (%dd)", lock, *a.RetentionPeriod)
			}
		}
		parts = append(parts, "locking: "+lock)
	} else {
		parts = append(parts, "locking: no")
	}
	return strings.Join(parts, "   ")
}

// describeCreatedBucket renders the detail lines under `make_bucket:`.
func describeCreatedBucket(b *objectstorage.Bucket) []string {
	first := []string{}
	if b.ID != "" {
		first = append(first, "id: "+b.ID)
	}
	if b.BucketName != "" {
		first = append(first, "bucket name on the endpoint: "+b.BucketName)
	}
	if b.StorageClass != "" {
		first = append(first, "class: "+b.StorageClass)
	}
	if b.Site != "" {
		first = append(first, "site: "+b.Site)
	}
	if b.Endpoint != "" {
		first = append(first, "endpoint: "+b.Endpoint)
	}
	lines := []string{strings.Join(first, "   ")}
	if b.Versioning || b.Locking {
		lines = append(lines, fmt.Sprintf("versioning: %s   locking: %s", yesNo(b.Versioning), lockingLabel(b)))
	}
	return lines
}

// mbPlan is the structured dry-run row of mb.
type mbPlan struct {
	Attributes operations.PostStorageBucketsAttributes
	DryRun     bool
}

func (p mbPlan) TableRow() table.Row {
	a := p.Attributes
	op := "make_bucket"
	if p.DryRun {
		op = "(dryrun) make_bucket"
	}
	retention := ""
	if a.RetentionMode != nil && *a.RetentionMode != operations.RetentionModeNone {
		retention = string(*a.RetentionMode)
		if a.RetentionPeriod != nil {
			retention = fmt.Sprintf("%s (%dd)", retention, *a.RetentionPeriod)
		}
	}
	return table.Row{
		"op":            {Label: "Op", Value: op},
		"name":          {Label: "Name", Value: a.Name},
		"project":       {Label: "Project", Value: a.Project},
		"region":        {Label: "Site", Value: a.Region},
		"storage_class": {Label: "Class", Value: string(*a.StorageClass)},
		"versioning":    {Label: "Versioning", Value: yesNo(a.Versioning != nil && *a.Versioning)},
		"locking":       {Label: "Locking", Value: yesNo(a.Locking != nil && *a.Locking)},
		"retention":     {Label: "Retention", Value: retention},
	}
}

// MarshalJSON emits the plan as a flat document.
func (p mbPlan) MarshalJSON() ([]byte, error) {
	a := p.Attributes
	doc := map[string]interface{}{
		"op":            "make_bucket",
		"dry_run":       p.DryRun,
		"name":          a.Name,
		"project":       a.Project,
		"region":        a.Region,
		"storage_class": string(*a.StorageClass),
		"versioning":    a.Versioning != nil && *a.Versioning,
		"locking":       a.Locking != nil && *a.Locking,
	}
	if a.RetentionMode != nil && *a.RetentionMode != operations.RetentionModeNone {
		doc["retention_mode"] = string(*a.RetentionMode)
		if a.RetentionPeriod != nil {
			doc["retention_days"] = *a.RetentionPeriod
		}
	}
	return json.Marshal(doc)
}

// pickRegion asks the user for a site when --region is missing, or fails
// with a usage error in non-interactive sessions.
func pickRegion(ctx context.Context, cmd *cobra.Command) (string, error) {
	if !objectstorage.CanPrompt(cmd) {
		return "", exitcode.Errorf(exitcode.Usage, "--region <site> is required; regions are Latitude sites such as DAL, NYC or TYO4 (run 'lsh regions list')")
	}
	sites, err := mbListSites(ctx)
	if err != nil {
		return "", err
	}
	if len(sites) == 0 {
		return "", exitcode.Errorf(exitcode.Usage, "--region <site> is required and no sites were returned by the API (run 'lsh regions list')")
	}
	options := make([]string, 0, len(sites))
	for _, s := range sites {
		options = append(options, s.label())
	}
	idx, err := objectstorage.Choose("Which site should host the bucket? (--region)", options, 0)
	if err != nil {
		return "", objectstorage.Humanize(err, nil, nil)
	}
	if idx < 0 {
		return "", exitcode.Errorf(exitcode.Refused, "no site selected; pass --region <site>")
	}
	return sites[idx].Slug, nil
}

type mbSite struct {
	Slug string
	Name string
}

func (s mbSite) label() string {
	if s.Name == "" || strings.EqualFold(s.Name, s.Slug) {
		return s.Slug
	}
	return s.Slug + " - " + s.Name
}

// mbListSites fetches the site slugs from the API (custom storage-only sites
// included), sorted by slug.
func mbListSites(ctx context.Context) ([]mbSite, error) {
	client := apiClient()
	resp, err := client.Regions.Get(ctx, operations.GetRegionsRequest{
		IncludeCustom: mbPtr(true),
		PageSize:      mbPtr(int64(100)),
	}, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return nil, objectstorage.Humanize(err, nil, nil)
	}
	var out []mbSite
	if resp != nil && resp.Regions != nil {
		for _, r := range resp.Regions.Data {
			if r.Attributes == nil || r.Attributes.Slug == nil || *r.Attributes.Slug == "" {
				continue
			}
			s := mbSite{Slug: *r.Attributes.Slug}
			if r.Attributes.Name != nil {
				s.Name = *r.Attributes.Name
			}
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// offerAccessKey implements the post-creation access key offer (J1). Any
// failure here is a warning: the bucket already exists and the command
// succeeds.
func offerAccessKey(ctx context.Context, cmd *cobra.Command, b *objectstorage.Bucket, o mbOptions) {
	f := cmd.Flags()
	noKey, _ := f.GetBool(flagMbNoAccessKey)
	createKey, _ := f.GetBool(flagMbCreateAccessKey)
	if noKey || endpointOverride(cmd) != "" {
		return
	}
	if !isHuman() {
		// The key would have to be printed to stay usable, and the structured
		// document of mb describes a bucket; say so instead of doing nothing.
		if createKey {
			objectstorage.Warnf("--create-access-key is only offered with human output; create the key with: lsh s3 access-keys create --bucket %s --save", b.Name)
		}
		return
	}
	if !createKey && !objectstorage.CanPrompt(cmd) {
		return
	}
	_, profileName, profile, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		objectstorage.Warnf("not saving an access key: %v", err)
		return
	}
	keys := profile.ObjectStorageKeys()
	if name, _, ok := objectstorage.SelectKey(keys, profile.DefaultObjectStorageKey(), b, true); ok {
		objectstorage.Hintf("Saved access key %q already covers this bucket; the CLI will use it automatically.", name)
		return
	}

	scope := config.ScopeFullAccess
	if !createKey {
		allLabel := fmt.Sprintf("key for all %s buckets of project %s", b.StorageClass, b.ProjectRef())
		if b.StorageClass == objectstorage.ClassHighPerformance && b.Site != "" {
			allLabel += " in site " + b.Site
		}
		idx, err := objectstorage.Choose(
			"To upload and download files the CLI needs an S3 access key (separate from your API token). Create one now?",
			[]string{allLabel + " (recommended)", "key for this bucket only (rw)", "skip"}, 0)
		if err != nil {
			objectstorage.Warnf("could not read the answer: %v", err)
			return
		}
		switch idx {
		case 0:
			scope = config.ScopeFullAccess
		case 1:
			scope = config.ScopeLimitedAccess
		default:
			objectstorage.Hintf("Skipped. Create a key later with: lsh s3 configure  (or: lsh s3 access-keys create --bucket %s --save)", b.Name)
			return
		}
	}

	if b.StorageClass == objectstorage.ClassHighPerformance && b.Site == "" {
		if err := newResolver(cmd).FillSite(ctx, b); err != nil {
			objectstorage.Warnf("could not determine the bucket's site: %v", err)
		}
	}
	// The name is generated the way the dashboard does it (see petname.go); a
	// clash with a saved key is handled by saveNewKey when storing.
	req := accessKeyRequest{
		Project:      firstNonEmptyStr(b.ProjectID, b.ProjectSlug, o.Project),
		StorageClass: b.StorageClass,
		Site:         b.Site,
		Scope:        scope,
	}
	var perms map[string]string
	switch scope {
	case config.ScopeLimitedAccess:
		perms = map[string]string{b.ID: config.PermissionRW}
		req.Buckets = perms
	}
	req.Name = generateKeyName(b.StorageClass)

	created, err := createAccessKeyRetrying(ctx, cmd, req, true)
	if err != nil {
		objectstorage.Warnf("bucket created, but the access key was not: %v", err)
		objectstorage.Hintf("Create one later with: lsh s3 access-keys create --bucket %s --save", b.Name)
		return
	}
	stored := created.stored(b.ProjectID, perms, config.KeySourceMakeBkt)
	if stored.StorageClass == "" {
		stored.StorageClass = b.StorageClass
	}
	if stored.Site == "" && b.StorageClass == objectstorage.ClassHighPerformance {
		stored.Site = b.Site
	}
	if stored.Scope == "" {
		stored.Scope = scope
	}
	stored.CreatedAt = time.Now().UTC()
	name := created.Name
	if name == "" {
		name = req.Name
	}
	finalName, savedProfile, err := saveNewKey(cmd, name, stored)
	if err != nil {
		mbReportUnsavedKey(ctx, os.Stderr, created, req, b, name, err)
		return
	}
	if savedProfile == "" {
		savedProfile = profileName
	}
	objectstorage.Hintf("Saved to %s (0600) as %q in profile %s. The CLI uses it automatically.", mbConfigPathForHint(), finalName, savedProfile)
	objectstorage.Hintf("For apps or CI create a separate key: lsh s3 access-keys create --bucket %s", b.Name)
}

// mbReportUnsavedKey handles a key that was created but could not be stored in
// the profile. mb creates the key on the user's behalf, so the key is removed
// again rather than leaving a credential whose secret would have to be printed;
// the shared helper only prints when the removal fails.
func mbReportUnsavedKey(ctx context.Context, w io.Writer, created *createdAccessKey, req accessKeyRequest, b *objectstorage.Bucket, name string, saveErr error) {
	created.Endpoint = firstNonEmptyStr(created.Endpoint, b.Endpoint)
	created.SigningRegion = firstNonEmptyStr(created.SigningRegion, b.SigningRegion)
	plan := &createPlan{Request: req}
	if req.Scope == config.ScopeLimitedAccess {
		plan.Buckets = []scopedBucket{{Bucket: b, Permission: config.PermissionRW}}
		created.Buckets = map[string]string{b.Name: config.PermissionRW}
	}
	reportUnsavedKey(ctx, w, req, created, plan, name, false, saveErr)
}

// mbConfigPathForHint renders the config location for messages.
func mbConfigPathForHint() string {
	if p := os.Getenv("LSH_CONFIG_PATH"); p != "" {
		return p
	}
	return "~/.config/lsh/config.json"
}
