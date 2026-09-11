package s3

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

// Flag names of `lsh s3 configure`.
const (
	flagCfgStorageClass = "storage-class"
	flagCfgRegion       = "region"
	flagCfgAllBuckets   = "all-buckets"
	flagCfgBucket       = "bucket"
	flagCfgName         = "name"
	flagCfgSave         = "save"
	flagCfgShowSecret   = "show-secret"
)

// configureNonInteractiveHint lists the alternatives to the wizard for
// scripts and CI (exit 2 when a prompt would be needed without a terminal).
const configureNonInteractiveHint = "pass every input as a flag (--project, --storage-class, --region, --all-buckets or --bucket <b>[=rw|readonly]), " +
	"or run 'lsh s3 access-keys create --bucket <b> --save', or set " + objectstorage.EnvAccessKeyID + "/" + objectstorage.EnvSecretAccessKey

// NewConfigureCmd builds `lsh s3 configure` (alias `credentials`): the
// interactive equivalent of `aws configure` for S3 access keys.
func NewConfigureCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "configure",
		Aliases: []string{"credentials"},
		GroupID: groupCredentials,
		Short:   "Create an access key and save it for this machine",
		Long: `Create an S3 access key and save it in the active lsh profile so the object
commands (list, copy, move, delete, get, presign) can use it automatically.

This is the path for the machine you are typing on. For an application, a CI
job or a colleague, create a separate key with 'lsh s3 access-keys create':
it prints the key once (and saves it here too with --save) instead of
walking through the wizard.

The wizard asks for the project, the storage class (standard = general
purpose; high_performance = low latency on selected sites), the site for
high_performance, the scope (all buckets of that class in the project, or
specific buckets with rw/readonly permission) and a name. Every question can
be answered with a flag; when all of them are given the wizard runs without
prompts, which is how scripts should call it. The site of a standard key is
inferred from the project's standard buckets when --region is omitted.

The access key is separate from your API token. It is stored in
~/.config/lsh/config.json (mode 0600) and never printed unless --save=false
is passed, in which case it is shown once in clear and not stored.`,
		Example: `  lsh s3 configure
  lsh s3 configure --project my-project --storage-class standard --all-buckets
  lsh s3 configure --project my-project --storage-class high_performance --region TYO4 --all-buckets
  lsh s3 configure --project my-project --bucket backups=rw --bucket logs=readonly --name laptop
  lsh s3 configure --project my-project --storage-class standard --all-buckets --save=false -o json`,
		Args: cobra.NoArgs,
		RunE: runConfigure,
	})
	addProjectFlag(cmd, true, "project ID or slug the key belongs to (default: LSH_PROJECT, or a menu)")
	cmd.Flags().String(flagCfgStorageClass, "", "storage class of the key: standard | high_performance")
	cmd.Flags().String(flagCfgRegion, "", "site slug (DAL, TYO4…) of the key; inferred from the project's buckets when omitted")
	cmd.Flags().Bool(flagCfgAllBuckets, false, "key for all buckets of the class (and site) in the project (fullaccess)")
	cmd.Flags().StringArray(flagCfgBucket, nil, "restrict the key to a bucket: <name|bkt_id>[=rw|readonly] (repeatable)")
	cmd.Flags().String(flagCfgName, "", "key name (default: lsh-<user>-<bucket|site|class>)")
	cmd.Flags().Bool(flagCfgSave, true, "save the key in the active profile; --save=false prints the secret once instead")
	cmd.Flags().Bool(flagCfgShowSecret, false, "include the secret in structured output even when -o comes from the environment")
	cmd.AddCommand(newConfigureExportCmd())
	return cmd
}

// configureOptions collects the flag values of `lsh s3 configure`.
type configureOptions struct {
	Project      string
	StorageClass string
	Site         string
	AllBuckets   bool
	Buckets      []bucketSpec
	Name         string
	Save         bool
}

// parseConfigureOptions reads and validates the flags.
func parseConfigureOptions(cmd *cobra.Command) (configureOptions, error) {
	o := configureOptions{Project: projectFlag(cmd)}
	rawClass, _ := cmd.Flags().GetString(flagCfgStorageClass)
	class, err := objectstorage.ParseStorageClass(rawClass)
	if err != nil {
		return o, err
	}
	o.StorageClass = class
	if o.Site, err = regionFlag(cmd); err != nil {
		return o, err
	}
	o.AllBuckets, _ = cmd.Flags().GetBool(flagCfgAllBuckets)
	values, _ := cmd.Flags().GetStringArray(flagCfgBucket)
	if o.Buckets, err = parseBucketSpecs(values); err != nil {
		return o, err
	}
	if o.AllBuckets && len(o.Buckets) > 0 {
		return o, objectstorage.ErrUsagef("--all-buckets and --bucket are mutually exclusive")
	}
	o.Name, _ = cmd.Flags().GetString(flagCfgName)
	o.Name = strings.TrimSpace(o.Name)
	o.Save, _ = cmd.Flags().GetBool(flagCfgSave)
	return o, nil
}

// cfgSelectedBucket is a bucket chosen for a limited_access key.
type cfgSelectedBucket struct {
	Bucket     *objectstorage.Bucket
	Permission string
}

// cfgValidateSelection checks that every selected bucket shares the storage
// class, site and project, which is what the API requires of one key. It
// returns the common values.
func cfgValidateSelection(sel []cfgSelectedBucket) (class, site, projectID string, err error) {
	if len(sel) == 0 {
		return "", "", "", objectstorage.ErrUsagef("no bucket selected")
	}
	first := sel[0].Bucket
	class, site, projectID = first.StorageClass, first.Site, first.ProjectID
	for _, s := range sel[1:] {
		b := s.Bucket
		if b.StorageClass != class {
			return "", "", "", objectstorage.ErrUsagef("buckets %s (%s) and %s (%s) have different storage classes; one key covers a single class",
				first.Display(), class, b.Display(), b.StorageClass)
		}
		if class == objectstorage.ClassHighPerformance && site != "" && b.Site != "" && !strings.EqualFold(site, b.Site) {
			return "", "", "", objectstorage.ErrUsagef("buckets %s (%s) and %s (%s) live on different sites; a high_performance key covers a single site",
				first.Display(), site, b.Display(), b.Site)
		}
		if projectID != "" && b.ProjectID != "" && projectID != b.ProjectID {
			return "", "", "", objectstorage.ErrUsagef("buckets %s (%s) and %s (%s) belong to different projects; one key covers a single project",
				first.Display(), first.ProjectRef(), b.Display(), b.ProjectRef())
		}
		if site == "" {
			site = b.Site
		}
		if projectID == "" {
			projectID = b.ProjectID
		}
	}
	return class, site, projectID, nil
}

// configurePlan is the structured description of the key the wizard creates
// (rendered for --dry-run and as the result when the key is saved).
type configurePlan struct {
	Name         string            `json:"name"`
	Project      string            `json:"project"`
	StorageClass string            `json:"storage_class"`
	Site         string            `json:"site,omitempty"`
	Scope        string            `json:"scope"`
	Buckets      map[string]string `json:"buckets,omitempty"`
	Save         bool              `json:"save"`
	DryRun       bool              `json:"dry_run,omitempty"`
	AccessKeyID  string            `json:"access_key_id,omitempty"`
	// SavedAs is the name the key was stored under, which differs from Name
	// when that name already held another key (saveNewKey appends -2, -3…).
	SavedAs string `json:"saved_as,omitempty"`
	Profile string `json:"profile,omitempty"`
}

func (p configurePlan) TableRow() table.Row {
	status := "created"
	if p.DryRun {
		status = "dryrun"
	}
	row := table.Row{
		"name":          {Label: "Name", Value: p.Name},
		"project":       {Label: "Project", Value: p.Project},
		"storage_class": {Label: "Class", Value: p.StorageClass},
		"site":          {Label: "Site", Value: orEmptyCell(p.Site)},
		"scope":         {Label: "Scope", Value: p.Scope},
		"buckets":       {Label: "Buckets", Value: cfgFormatBucketPerms(p.Buckets), MaxLength: 60},
		"save":          {Label: "Saved", Value: yesNo(p.Save && !p.DryRun)},
		"status":        {Label: "Status", Value: status},
	}
	if p.AccessKeyID != "" {
		row["access_key_id"] = table.Cell{Label: "Access Key ID", Value: p.AccessKeyID}
	}
	if p.SavedAs != "" {
		row["saved_as"] = table.Cell{Label: "Saved As", Value: p.SavedAs}
	}
	return row
}

// cfgFormatBucketPerms renders "a=rw b=readonly" sorted by name (the shared
// empty-cell placeholder when empty).
func cfgFormatBucketPerms(perms map[string]string) string {
	if len(perms) == 0 {
		return emptyCell
	}
	parts := make([]string, 0, len(perms))
	for name, perm := range perms {
		parts = append(parts, name+"="+perm)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// configureSecretResult is the --save=false output: the key shown once.
type configureSecretResult struct {
	Name            string            `json:"name"`
	AccessKeyID     string            `json:"access_key_id"`
	SecretAccessKey *string           `json:"secret_access_key"`
	Scope           string            `json:"scope"`
	StorageClass    string            `json:"storage_class"`
	Site            string            `json:"site,omitempty"`
	Project         string            `json:"project"`
	Endpoint        string            `json:"endpoint,omitempty"`
	SigningRegion   string            `json:"signing_region,omitempty"`
	Buckets         map[string]string `json:"buckets,omitempty"`
}

func (r configureSecretResult) TableRow() table.Row {
	return table.Row{
		"name":          {Label: "Name", Value: r.Name},
		"access_key_id": {Label: "Access Key ID", Value: r.AccessKeyID},
		"scope":         {Label: "Scope", Value: r.Scope},
		"storage_class": {Label: "Class", Value: r.StorageClass},
		"site":          {Label: "Site", Value: orEmptyCell(r.Site)},
		"project":       {Label: "Project", Value: r.Project},
		"buckets":       {Label: "Buckets", Value: cfgFormatBucketPerms(r.Buckets), MaxLength: 60},
	}
}

// configureNeedsPrompt returns the exit-2 error for a question the wizard
// cannot ask without a terminal.
func configureNeedsPrompt(what string) error {
	return exitcode.Errorf(exitcode.Usage, "lsh s3 configure needs a terminal to ask for %s; %s", what, configureNonInteractiveHint)
}

// cfgProjectChoice is one project offered by the wizard.
type cfgProjectChoice struct {
	ID   string
	Slug string
	Name string
}

// configureListProjects fetches the team's projects (all pages, capped).
func configureListProjects(ctx context.Context, api *sdk.Latitudesh) ([]cfgProjectChoice, error) {
	size := int64(100)
	resp, err := api.Projects.List(ctx, operations.GetProjectsRequest{PageSize: &size}, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, "projects")
	}
	var out []cfgProjectChoice
	for page := 0; resp != nil && resp.Projects != nil && page < 20; page++ {
		for _, p := range resp.Projects.Data {
			c := cfgProjectChoice{ID: ptrStr(p.ID)}
			if p.Attributes != nil {
				c.Slug = ptrStr(p.Attributes.Slug)
				c.Name = ptrStr(p.Attributes.Name)
			}
			if c.ID != "" {
				out = append(out, c)
			}
		}
		if resp.Next == nil {
			break
		}
		next, err := resp.Next()
		if err != nil {
			return nil, objectstorage.HumanizeAPI(err, "projects")
		}
		if next == nil {
			break
		}
		resp = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].label() < out[j].label() })
	return out, nil
}

func (p cfgProjectChoice) label() string {
	switch {
	case p.Slug != "" && p.Name != "" && p.Slug != p.Name:
		return fmt.Sprintf("%s (%s)", p.Slug, p.Name)
	case p.Slug != "":
		return p.Slug
	case p.Name != "":
		return p.Name
	}
	return p.ID
}

// cfgResolveProjectID turns a project token (ID or slug) into the proj_ ID the
// saved key is bound to. Unknown tokens are returned unchanged.
func cfgResolveProjectID(ctx context.Context, api *sdk.Latitudesh, token string) string {
	if token == "" || strings.HasPrefix(token, "proj_") || api == nil {
		return token
	}
	projects, err := configureListProjects(ctx, api)
	if err != nil {
		return token
	}
	for _, p := range projects {
		if p.Slug == token {
			return p.ID
		}
	}
	for _, p := range projects {
		if p.Name == token {
			return p.ID
		}
	}
	return token
}

// cfgSiteChoice is one region offered by the wizard.
type cfgSiteChoice struct {
	Slug    string
	Name    string
	Country string
}

func (s cfgSiteChoice) label() string {
	return joinNonEmpty(" — ", s.Slug, joinNonEmpty(", ", s.Name, s.Country))
}

// configureListSites fetches every region (including storage-only ones).
func configureListSites(ctx context.Context, api *sdk.Latitudesh) ([]cfgSiteChoice, error) {
	size := int64(100)
	includeCustom := true
	resp, err := api.Regions.Get(ctx, operations.GetRegionsRequest{IncludeCustom: &includeCustom, PageSize: &size}, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, "regions")
	}
	var out []cfgSiteChoice
	for page := 0; resp != nil && resp.Regions != nil && page < 20; page++ {
		for _, r := range resp.Regions.Data {
			if r.Attributes == nil {
				continue
			}
			c := cfgSiteChoice{Slug: ptrStr(r.Attributes.Slug), Name: ptrStr(r.Attributes.Name)}
			if r.Attributes.Country != nil {
				c.Country = ptrStr(r.Attributes.Country.Name)
			}
			if c.Slug != "" {
				out = append(out, c)
			}
		}
		if resp.Next == nil {
			break
		}
		next, err := resp.Next()
		if err != nil {
			return nil, objectstorage.HumanizeAPI(err, "regions")
		}
		if next == nil {
			break
		}
		resp = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// configureState is the wizard's accumulated answers.
type configureState struct {
	ProjectToken string // as given (ID or slug)
	ProjectID    string
	StorageClass string
	Site         string
	Scope        string
	Selected     []cfgSelectedBucket
	Name         string
	// Endpoint/SigningRegion are known when specific buckets were selected.
	Endpoint      string
	SigningRegion string
}

// project returns the best project reference known so far.
func (s *configureState) project() string {
	return firstNonEmptyStr(s.ProjectToken, s.ProjectID)
}

// bucketPerms returns bkt_ ID → permission for a limited selection.
func (s *configureState) bucketPerms() map[string]string {
	if s.Scope != config.ScopeLimitedAccess {
		return nil
	}
	out := map[string]string{}
	for _, sel := range s.Selected {
		out[sel.Bucket.ID] = sel.Permission
	}
	return out
}

// bucketNamePerms returns display name → permission for messages.
func (s *configureState) bucketNamePerms() map[string]string {
	if s.Scope != config.ScopeLimitedAccess {
		return nil
	}
	out := map[string]string{}
	for _, sel := range s.Selected {
		out[sel.Bucket.Name] = sel.Permission
	}
	return out
}

func runConfigure(cmd *cobra.Command, _ []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	opts, err := parseConfigureOptions(cmd)
	if err != nil {
		return printErr(err)
	}

	canPrompt := objectstorage.CanPrompt(cmd)
	api := apiClient()

	// Saving needs an active profile; check before creating anything so a
	// LATITUDESH_TOKEN-only session does not end up with an unsaved key.
	if opts.Save {
		if _, _, _, err := objectstorage.ActiveProfile(profileFlag(cmd)); err != nil {
			return printErr(exitcode.Errorf(exitcode.Of(err), "%v\n  configure saves the key in a profile; run 'lsh login' first, or pass --save=false to print the key instead", err))
		}
	}

	st := &configureState{StorageClass: opts.StorageClass, Site: opts.Site}

	// Step 4 first when --bucket was given: the buckets pin the project,
	// class and site, so nothing else has to be asked.
	if len(opts.Buckets) > 0 {
		if err := configureSelectFromSpecs(ctx, cmd, st, opts); err != nil {
			return printErr(err)
		}
	}

	// Step 1: project.
	if st.ProjectID == "" {
		st.ProjectToken = opts.Project
		if st.ProjectToken == "" {
			if !canPrompt {
				return printErr(configureNeedsPrompt("the project (--project or LSH_PROJECT)"))
			}
			projects, err := configureListProjects(ctx, api)
			if err != nil {
				return printErr(err)
			}
			if len(projects) == 0 {
				return printErr(exitcode.Errorf(exitcode.NotFound, "no projects found; create one with 'lsh projects create'"))
			}
			labels := make([]string, len(projects))
			for i, p := range projects {
				labels[i] = p.label()
			}
			idx, err := objectstorage.Choose("Which project should the key belong to?", labels, 0)
			if err != nil {
				return printErr(err)
			}
			if idx < 0 {
				return printErr(exitcode.Errorf(exitcode.Refused, "cancelled"))
			}
			st.ProjectToken, st.ProjectID = projects[idx].ID, projects[idx].ID
		} else {
			st.ProjectID = cfgResolveProjectID(ctx, api, st.ProjectToken)
		}
	}

	// Step 2: storage class.
	if st.StorageClass == "" {
		if !canPrompt {
			return printErr(configureNeedsPrompt("the storage class (--storage-class standard|high_performance)"))
		}
		idx, err := objectstorage.Choose("Which storage class?", []string{
			"standard — general purpose object storage, one key covers every site of the project",
			"high_performance — low latency storage on selected sites, one key per site",
		}, 0)
		if err != nil {
			return printErr(err)
		}
		if idx < 0 {
			return printErr(exitcode.Errorf(exitcode.Refused, "cancelled"))
		}
		st.StorageClass = []string{objectstorage.ClassStandard, objectstorage.ClassHighPerformance}[idx]
	}

	// Step 3: site. A high_performance key is bound to one cluster, so the
	// site is asked up front (it also filters the bucket menu below).
	if st.StorageClass == objectstorage.ClassHighPerformance && st.Site == "" {
		if err := configureAskSite(ctx, api, st, canPrompt, "the site of a high_performance key"); err != nil {
			return printErr(err)
		}
	}

	// Step 4: scope.
	if st.Scope == "" {
		switch {
		case opts.AllBuckets:
			st.Scope = config.ScopeFullAccess
		case !canPrompt:
			return printErr(configureNeedsPrompt("the scope (--all-buckets or --bucket <b>[=rw|readonly])"))
		default:
			where := st.StorageClass + " buckets"
			if st.Site != "" {
				where += " in " + st.Site
			}
			idx, err := objectstorage.Choose("Which buckets should the key cover?", []string{
				"all " + where + " of the project, including future ones (fullaccess, recommended)",
				"specific buckets (limited_access, rw or readonly)",
			}, 0)
			if err != nil {
				return printErr(err)
			}
			switch idx {
			case 0:
				st.Scope = config.ScopeFullAccess
			case 1:
				if err := configureSelectInteractive(ctx, cmd, st); err != nil {
					return printErr(err)
				}
			default:
				return printErr(exitcode.Errorf(exitcode.Refused, "cancelled"))
			}
		}
	}

	// A fullaccess key still needs a site (the API requires one). Without
	// --region it is inferred from the project's buckets of that class, the way
	// 'access-keys create --all-buckets' does; a standard key covers every site
	// so any site of a standard bucket will do.
	if st.Scope == config.ScopeFullAccess && st.Site == "" {
		site, err := configureInferSite(ctx, cmd, st)
		if err != nil {
			return printErr(err)
		}
		if site == "" {
			what := fmt.Sprintf("the site of the key (project %s has no %s buckets to infer it from)", st.project(), st.StorageClass)
			if err := configureAskSite(ctx, api, st, canPrompt, what); err != nil {
				return printErr(err)
			}
		} else {
			st.Site = site
		}
	}

	// Step 5: name.
	st.Name = opts.Name
	if st.Name == "" {
		st.Name = generateKeyName(st.StorageClass)
		if canPrompt && !dryRun() {
			answer, err := objectstorage.ReadLine(fmt.Sprintf("Key name [%s]: ", st.Name))
			if err == nil && answer != "" {
				st.Name = answer
			}
		}
	}

	plan := configurePlan{
		Name:         st.Name,
		Project:      st.project(),
		StorageClass: st.StorageClass,
		Site:         st.Site,
		Scope:        st.Scope,
		Buckets:      st.bucketNamePerms(),
		Save:         opts.Save,
		DryRun:       dryRun(),
	}

	// Dry-run: describe, never create.
	if dryRun() {
		if isHuman() {
			fmt.Println(configurePlanLine(plan))
		} else {
			render([]renderer.ResponseData{plan})
		}
		return nil
	}

	// Step 6: create (and save). When the name was auto-generated (the user did
	// not pass --name), let a collision re-roll a fresh pet name.
	created, err := createAccessKeyRetrying(ctx, cmd, accessKeyRequest{
		Project:      firstNonEmptyStr(st.ProjectID, st.ProjectToken),
		StorageClass: st.StorageClass,
		Site:         st.Site,
		Name:         st.Name,
		Scope:        st.Scope,
		Buckets:      st.bucketPerms(),
	}, opts.Name == "")
	if err != nil {
		return printErr(objectstorage.HumanizeAPI(err, "access key"))
	}
	if created.Name != "" {
		// The API normalizes names; keep the server's spelling as the index.
		st.Name = created.Name
		plan.Name = created.Name
	}
	plan.AccessKeyID = created.AccessKeyID

	if !opts.Save {
		return configurePrintSecret(cmd, st, created)
	}

	savedAs, profileName, err := saveNewKey(cmd, st.Name, created.stored(st.ProjectID, st.bucketPerms(), config.KeySourceConfigure))
	if err != nil {
		// configure creates the key on the user's behalf, so a key it cannot
		// store is removed again instead of having its one-time secret
		// printed. --show-secret opts into keeping and printing it.
		showSecret, _ := cmd.Flags().GetBool(flagCfgShowSecret)
		if !showSecret {
			req := accessKeyRequest{Project: firstNonEmptyStr(st.ProjectID, st.ProjectToken), StorageClass: st.StorageClass, Site: st.Site, Scope: st.Scope}
			if delErr := discardKey(ctx, req, created); delErr == nil {
				return printErr(exitcode.Errorf(exitcode.Generic,
					"access key %q was created but could not be saved: %v — it was deleted again, so nothing was left behind; retry once the profile is writable", st.Name, err))
			}
			objectstorage.Warnf("access key %q could not be saved (%v) nor deleted again, so its secret is shown below", st.Name, err)
			return configurePrintSecret(cmd, st, created)
		}
		objectstorage.Warnf("access key %q was created but could not be saved: %v", st.Name, err)
		return configurePrintSecret(cmd, st, created)
	}
	plan.Profile, plan.SavedAs = profileName, savedAs
	path, _ := config.Path()
	if path == "" {
		path = "~/.config/lsh/config.json"
	}
	objectstorage.Hintf("Saved to %s (0600) as %q in profile %s. The CLI uses it automatically.", path, savedAs, profileName)
	if savedAs != st.Name {
		objectstorage.Hintf("(%q already held another key, so this one was saved as %q)", st.Name, savedAs)
	}
	objectstorage.Hintf("Next: lsh s3 list%s", configureExampleBucket(st, "   lsh s3 copy ./file s3://%s/"))
	objectstorage.Hintf("For apps or CI create a separate key: lsh s3 access-keys create --bucket <b> --name <app>")
	objectstorage.Hintf("Export it for other tools: lsh s3 configure export --format env%s", configureExampleBucket(st, " s3://%s"))
	if !isHuman() {
		render([]renderer.ResponseData{plan})
	}
	return nil
}

// configureAskSite fills st.Site from a menu of Latitude sites, or fails with
// exit 2 naming --region when no terminal is available. what describes the
// value being asked for in the error message.
func configureAskSite(ctx context.Context, api *sdk.Latitudesh, st *configureState, canPrompt bool, what string) error {
	sites, listErr := configureListSites(ctx, api)
	if !canPrompt {
		slugs := make([]string, 0, len(sites))
		for _, s := range sites {
			slugs = append(slugs, s.Slug)
		}
		hint := ""
		if len(slugs) > 0 {
			hint = "; available: " + strings.Join(slugs, ", ")
		}
		return configureNeedsPrompt(what + " (--region <site>" + hint + ")")
	}
	if listErr != nil {
		return listErr
	}
	if len(sites) == 0 {
		return exitcode.Errorf(exitcode.NotFound, "no sites returned by the API; pass --region <site>")
	}
	labels := make([]string, len(sites))
	for i, s := range sites {
		labels[i] = s.label()
	}
	question := "Which site should the key be created in?"
	if st.StorageClass == objectstorage.ClassHighPerformance {
		question = "Which site is the high_performance cluster on?"
	}
	idx, err := objectstorage.Choose(question, labels, 0)
	if err != nil {
		return err
	}
	if idx < 0 {
		return exitcode.Errorf(exitcode.Refused, "cancelled")
	}
	st.Site = strings.ToUpper(sites[idx].Slug)
	return nil
}

// configureInferSite lists the project's buckets and returns the site for a
// fullaccess key of st.StorageClass, or "" when the project has no bucket of
// that class. Sites come from the raw API because the SDK model drops them.
func configureInferSite(ctx context.Context, cmd *cobra.Command, st *configureState) (string, error) {
	r := newResolver(cmd)
	r.Project = st.project()
	if r.EndpointURL != "" {
		return "", objectstorage.ErrUsagef("configure needs the Latitude API; unset --endpoint-url/%s", objectstorage.EnvEndpointURL)
	}
	list, err := r.ListBuckets(ctx)
	if err != nil {
		return "", err
	}
	buckets := make([]*objectstorage.Bucket, 0, len(list))
	for _, d := range list {
		buckets = append(buckets, objectstorage.BucketFromData(d))
	}
	var sites map[string]string
	if len(buckets) > 0 {
		if sites, err = objectstorage.RawBucketSitesForProject(ctx, "", r.Project); err != nil {
			lsh.LogDebugf("[s3] site lookup failed for project %s: %v", r.Project, err)
		}
	}
	return configureSiteFromBuckets(st.StorageClass, r.Project, buckets, sites)
}

// configureSiteFromBuckets picks the site for a fullaccess key from the
// project's buckets of the given class (sites supplies bkt_ ID → site for
// buckets whose Site is empty). A standard key covers every site, so the
// first site in sorted order is used; several high_performance sites are
// ambiguous and rejected with exit 2. "" means nothing to infer from.
func configureSiteFromBuckets(class, project string, buckets []*objectstorage.Bucket, sites map[string]string) (string, error) {
	distinct := map[string]bool{}
	for _, b := range buckets {
		if b.StorageClass != class {
			continue
		}
		if s := strings.ToUpper(firstNonEmptyStr(b.Site, sites[b.ID])); s != "" {
			distinct[s] = true
		}
	}
	if len(distinct) == 0 {
		return "", nil
	}
	names := make([]string, 0, len(distinct))
	for s := range distinct {
		names = append(names, s)
	}
	sort.Strings(names)
	if class == objectstorage.ClassHighPerformance && len(names) > 1 {
		return "", objectstorage.ErrUsagef("project %s has high_performance buckets in several sites (%s); pass --region <site>", project, strings.Join(names, ", "))
	}
	return names[0], nil
}

// configureExampleBucket formats an example with the first selected bucket,
// or "" when the key is fullaccess (no bucket to point at).
func configureExampleBucket(st *configureState, format string) string {
	if len(st.Selected) == 0 || st.Selected[0].Bucket.Name == "" {
		return ""
	}
	return fmt.Sprintf(format, st.Selected[0].Bucket.Name)
}

// configurePlanLine renders the dry-run plan as one human line.
func configurePlanLine(p configurePlan) string {
	scope := "all " + p.StorageClass + " buckets"
	if p.Site != "" {
		scope += " in " + p.Site
	}
	scope += " (fullaccess)"
	if p.Scope == config.ScopeLimitedAccess {
		scope = "buckets " + cfgFormatBucketPerms(p.Buckets) + " (limited_access)"
	}
	action := "save it in the active profile"
	if !p.Save {
		action = "print the secret once (not saved)"
	}
	return fmt.Sprintf("(dryrun) create access key %q in project %s for %s and %s", p.Name, p.Project, scope, action)
}

// configurePrintSecret shows the created key once, in clear (--save=false).
// Structured output embeds the secret only when -o/--json was given on the
// command line (never because of LSH_OUTPUT or the config file).
func configurePrintSecret(cmd *cobra.Command, st *configureState, created *createdAccessKey) error {
	res := configureSecretResult{
		Name:          st.Name,
		AccessKeyID:   created.AccessKeyID,
		Scope:         st.Scope,
		StorageClass:  st.StorageClass,
		Site:          st.Site,
		Project:       st.project(),
		Endpoint:      st.Endpoint,
		SigningRegion: st.SigningRegion,
		Buckets:       st.bucketNamePerms(),
	}
	showSecret, _ := cmd.Flags().GetBool(flagCfgShowSecret)
	decision := decideSecretDisplay(isHuman(), false, showSecret, outputExplicit(cmd))
	if isHuman() {
		fmt.Printf("Access key %q created. The secret is shown once and cannot be retrieved again:\n", res.Name)
		fmt.Printf("  Access Key ID:      %s\n", created.AccessKeyID)
		fmt.Printf("  Secret Access Key:  %s\n", created.SecretAccessKey)
		if res.Endpoint != "" {
			fmt.Printf("  Endpoint:           %s   Signing region: %s\n", res.Endpoint, res.SigningRegion)
		}
		if len(res.Buckets) > 0 {
			fmt.Printf("  Buckets:            %s\n", cfgFormatBucketPerms(res.Buckets))
		} else {
			fmt.Printf("  Scope:              all %s buckets of the project%s\n", res.StorageClass, cfgSiteSuffix(res.Site))
		}
		objectstorage.Hintf("Save it later with: lsh s3 access-keys import --name %s --access-key-id %s", res.Name, created.AccessKeyID)
		return nil
	}
	if decision.Show {
		secret := created.SecretAccessKey
		res.SecretAccessKey = &secret
	} else {
		// The key exists on the API and the secret is returned exactly once, so
		// it cannot simply be dropped: stdout stays clean (it was not asked for
		// explicitly and CI captures it) and the secret goes to stderr.
		objectstorage.Warnf("secret_access_key omitted from stdout: structured output came from LSH_OUTPUT/config, not -o; pass -o json or --show-secret to include it. The secret is printed on stderr below — it cannot be retrieved again.")
		fmt.Fprintf(os.Stderr, "  Access Key ID:      %s\n", created.AccessKeyID)
		fmt.Fprintf(os.Stderr, "  Secret Access Key:  %s\n", created.SecretAccessKey)
		fmt.Fprintf(os.Stderr, "  Save it with: lsh s3 access-keys import --name %s --access-key-id %s\n", res.Name, created.AccessKeyID)
	}
	render([]renderer.ResponseData{res})
	return nil
}

func cfgSiteSuffix(site string) string {
	if site == "" {
		return ""
	}
	return " in " + site
}

// configureSelectFromSpecs resolves the --bucket values, validates them and
// fills the state (class, site, project, scope) from the buckets.
func configureSelectFromSpecs(ctx context.Context, cmd *cobra.Command, st *configureState, opts configureOptions) error {
	r := newResolver(cmd)
	if r.EndpointURL != "" {
		return objectstorage.ErrUsagef("configure needs the Latitude API; unset --endpoint-url/%s", objectstorage.EnvEndpointURL)
	}
	sel := make([]cfgSelectedBucket, 0, len(opts.Buckets))
	for _, spec := range opts.Buckets {
		b, err := r.Resolve(ctx, spec.Token)
		if err != nil {
			return err
		}
		if err := r.FillSite(ctx, b); err != nil {
			lsh.LogDebugf("[s3] could not fetch site of %s: %v", b.Display(), err)
		}
		sel = append(sel, cfgSelectedBucket{Bucket: b, Permission: spec.Permission})
	}
	class, site, projectID, err := cfgValidateSelection(sel)
	if err != nil {
		return err
	}
	if opts.StorageClass != "" && opts.StorageClass != class {
		return objectstorage.ErrUsagef("--storage-class %s does not match the selected buckets (%s)", opts.StorageClass, class)
	}
	if st.Site != "" && site != "" && !strings.EqualFold(st.Site, site) {
		return objectstorage.ErrUsagef("--region %s does not match the selected buckets (%s)", st.Site, site)
	}
	if opts.Project != "" && projectID != "" && opts.Project != projectID && !anyProjectSlugMatches(sel, opts.Project) {
		return objectstorage.ErrUsagef("--project %s does not match the selected buckets (%s)", opts.Project, projectID)
	}
	st.StorageClass = class
	if site != "" {
		st.Site = strings.ToUpper(site)
	}
	st.ProjectID = projectID
	st.ProjectToken = firstNonEmptyStr(opts.Project, sel[0].Bucket.ProjectRef(), projectID)
	st.Scope = config.ScopeLimitedAccess
	st.Selected = sel
	st.Endpoint = sel[0].Bucket.Endpoint
	st.SigningRegion = sel[0].Bucket.SigningRegion
	if st.StorageClass == objectstorage.ClassHighPerformance && st.Site == "" {
		return objectstorage.ErrUsagef("could not determine the site of the selected high_performance buckets; pass --region <site>")
	}
	return nil
}

func anyProjectSlugMatches(sel []cfgSelectedBucket, token string) bool {
	for _, s := range sel {
		if s.Bucket.ProjectSlug == token || s.Bucket.ProjectName == token {
			return true
		}
	}
	return false
}

// configureSelectInteractive lists the buckets of the chosen class (and site)
// in the project and lets the user pick some by number, then a permission.
func configureSelectInteractive(ctx context.Context, cmd *cobra.Command, st *configureState) error {
	r := newResolver(cmd)
	r.Project = st.project()
	if r.EndpointURL != "" {
		return objectstorage.ErrUsagef("configure needs the Latitude API; unset --endpoint-url/%s", objectstorage.EnvEndpointURL)
	}
	data, err := r.ListBuckets(ctx)
	if err != nil {
		return err
	}
	var sites map[string]string
	if st.StorageClass == objectstorage.ClassHighPerformance {
		sites, err = objectstorage.RawBucketSitesForProject(ctx, "", r.Project)
		if err != nil {
			lsh.LogDebugf("[s3] could not fetch bucket sites: %v", err)
			sites = nil
		}
	}
	var candidates []*objectstorage.Bucket
	for _, d := range data {
		b := objectstorage.BucketFromData(d)
		if b.StorageClass != st.StorageClass {
			continue
		}
		if b.Site == "" {
			b.Site = sites[b.ID]
		}
		if st.StorageClass == objectstorage.ClassHighPerformance && st.Site != "" && b.Site != "" && !strings.EqualFold(b.Site, st.Site) {
			continue
		}
		candidates = append(candidates, b)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	if len(candidates) == 0 {
		where := st.StorageClass
		if st.Site != "" {
			where += " in " + st.Site
		}
		return exitcode.Errorf(exitcode.NotFound, "no %s buckets found in project %s; create one with 'lsh s3 create-bucket s3://<name> --region <site> --project %s'", where, r.Project, r.Project)
	}
	labels := make([]string, len(candidates))
	for i, b := range candidates {
		labels[i] = fmt.Sprintf("%s  (%s, %s)", b.Name, b.ID, joinNonEmpty(" ", b.StorageClass, firstNonEmptyStr(b.Site, b.City)))
	}
	idx, err := objectstorage.ChooseMany("Which buckets should the key cover?", labels)
	if err != nil {
		return err
	}
	if len(idx) == 0 {
		return exitcode.Errorf(exitcode.Refused, "no bucket selected; pick at least one with space, or pass --bucket <name>")
	}
	permIdx, err := objectstorage.Choose("Which permission on these buckets?", []string{
		"rw — read and write",
		"readonly — list and download only",
	}, 0)
	if err != nil {
		return err
	}
	if permIdx < 0 {
		return exitcode.Errorf(exitcode.Refused, "cancelled")
	}
	perm := []string{config.PermissionRW, config.PermissionReadOnly}[permIdx]
	sel := make([]cfgSelectedBucket, 0, len(idx))
	for _, i := range idx {
		sel = append(sel, cfgSelectedBucket{Bucket: candidates[i], Permission: perm})
	}
	class, site, projectID, err := cfgValidateSelection(sel)
	if err != nil {
		return err
	}
	st.StorageClass = class
	if site != "" {
		st.Site = strings.ToUpper(site)
	}
	if projectID != "" {
		st.ProjectID = projectID
	}
	st.Scope = config.ScopeLimitedAccess
	st.Selected = sel
	st.Endpoint = sel[0].Bucket.Endpoint
	st.SigningRegion = sel[0].Bucket.SigningRegion
	return nil
}
