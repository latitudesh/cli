package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	cobra "github.com/spf13/cobra"
)

func newAccessKeysListCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List access keys (API), or the keys saved in this profile (--saved)",
		Long: `List the access keys of a project. Pick a project with --project (or
LSH_PROJECT), list every project with --all-projects, or — in a terminal —
choose from the project picker (which includes "All projects").

SCOPE is fullaccess, rw or readonly as the API reports it; BUCKETS are the
backend bucket names the key covers; SAVED tells whether the key (by access key
ID) is stored in the active profile. Secrets are never listed.

--saved lists only the local profile, marking keys the API no longer knows as
"missing on API".`,
		Example: `  lsh s3 access-keys list
  lsh s3 access-keys list --project my-project -o json
  lsh s3 access-keys list --bucket backups
  lsh s3 access-keys list --saved`,
		Args: cobra.NoArgs,
		RunE: runAccessKeysList,
	})
	addProjectFlag(cmd, true, "list the keys of this project only (ID or slug)")
	cmd.Flags().Bool("all-projects", false, "list the keys of every project (skip the project picker)")
	cmd.Flags().StringP("storage-class", "c", "", "only keys of this storage class: standard or high_performance")
	cmd.Flags().String("region", "", "only keys of this site (e.g. DAL, TYO4)")
	cmd.Flags().String("bucket", "", "only the keys covering this bucket (name or bkt_ ID)")
	cmd.Flags().Bool("saved", false, "list the keys saved in the active profile instead of the API")
	return cmd
}

// listFilter narrows a key listing.
type listFilter struct {
	StorageClass string
	Site         string
	Project      string
}

// matchAPI applies the class and site filters. A key the API lists without
// a site (standard keys span sites) matches any --region: the flag then
// disambiguates rather than hides.
func (f listFilter) matchAPI(k apiKey) bool {
	if f.StorageClass != "" && k.StorageClass != f.StorageClass {
		return false
	}
	if f.Site != "" && k.Site != "" && !strings.EqualFold(k.Site, f.Site) {
		return false
	}
	return true
}

// withSite returns k with the --region value filled in when the API left the
// site empty, so a single match carries the site the user named.
func (f listFilter) withSite(k apiKey) apiKey {
	if k.Site == "" && f.Site != "" {
		k.Site = f.Site
	}
	return k
}

func (f listFilter) matchSaved(k config.StoredAccessKey) bool {
	if f.StorageClass != "" && k.StorageClass != f.StorageClass {
		return false
	}
	// Same wildcard as matchAPI: a key stored without a site (standard keys
	// span sites) matches any --region instead of being hidden by it.
	if f.Site != "" && k.Site != "" && !strings.EqualFold(k.Site, f.Site) {
		return false
	}
	// --project takes an ID or a slug, while the key stores whichever was
	// known when it was saved. Comparing across the two kinds would hide keys
	// instead of filtering them, so only like-for-like counts as a mismatch.
	if f.Project != "" && k.ProjectID != "" && comparableProjects(k.ProjectID, f.Project) && k.ProjectID != f.Project {
		return false
	}
	return true
}

// comparableProjects reports whether two project references are of the same
// kind (both proj_ IDs or both slugs) and can therefore be compared.
func comparableProjects(a, b string) bool {
	return strings.HasPrefix(a, "proj_") == strings.HasPrefix(b, "proj_")
}

func runAccessKeysList(cmd *cobra.Command, _ []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	class, err := storageClassFlag(cmd)
	if err != nil {
		return printErr(err)
	}
	site, err := regionFlag(cmd)
	if err != nil {
		return printErr(err)
	}
	filter := listFilter{StorageClass: class, Site: site, Project: projectFlag(cmd)}

	if saved, _ := cmd.Flags().GetBool("saved"); saved {
		return runAccessKeysListSaved(ctx, cmd, filter)
	}

	api := newKeysAPI()
	r := newResolver(cmd)
	var keys []apiKey
	slugs := map[string]string{}
	multiProject := false
	if bucket, _ := cmd.Flags().GetString("bucket"); bucket != "" {
		b, err := r.Resolve(ctx, bucket)
		if err != nil {
			return printErr(err)
		}
		if b.EndpointOverride {
			return printErr(exitcode.Errorf(exitcode.Usage, "access keys are managed through the Latitude API; unset --endpoint-url"))
		}
		if err := r.FillSite(ctx, b); err != nil {
			lsh.LogDebugf("[s3] site lookup failed for %s: %v", b.ID, err)
		}
		keys, err = api.listForBucket(ctx, b)
		if err != nil {
			return printErr(err)
		}
		if b.ProjectSlug != "" {
			slugs[b.ProjectRef()] = b.ProjectSlug
		}
	} else {
		// Pick the project like `lsh s3 list`: --project / LSH_PROJECT, or
		// --all-projects, otherwise the interactive picker (with "All projects").
		project, allProjects, perr := cli.PickProjectForList(cmd)
		if perr != nil {
			return printErr(perr)
		}
		var projects []string
		if allProjects {
			projects, slugs, err = teamProjects(ctx, r)
			if err != nil {
				return printErr(err)
			}
			multiProject = len(projects) > 1
		} else {
			projects = []string{project}
			filter.Project = project
		}
		keys, err = api.listProjects(ctx, projects)
		if err != nil {
			return printErr(err)
		}
	}

	savedByID, _, _ := savedKeysByID(cmd)
	var rows []keyRow
	for _, k := range keys {
		if !filter.matchAPI(k) {
			continue
		}
		rows = append(rows, newKeyRow(k, savedByID, slugs))
	}
	if isHuman() {
		if len(rows) == 0 {
			objectstorage.Hintf("no access keys found")
			return nil
		}
		writeKeyTable(os.Stdout, rows, multiProject)
		return nil
	}
	out := make([]renderer.ResponseData, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	render(out)
	return nil
}

// writeKeyTable prints the human listing: an aligned table with a header, no
// borders, the way the plan shows it.
func writeKeyTable(w io.Writer, rows []keyRow, withProject bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	header := "NAME\tACCESS KEY ID\tSCOPE\tBUCKETS\tCLASS\tSITE\tSTATUS\tSAVED"
	if withProject {
		header = "PROJECT\t" + header
	}
	fmt.Fprintln(tw, header)
	for _, r := range rows {
		line := strings.Join([]string{r.Name, r.AccessKeyID, r.scopeCell(), r.bucketsCell(), r.StorageClass, dash(r.Site), dash(r.Status), yesNo(r.Saved)}, "\t")
		if withProject {
			line = dash(r.Project) + "\t" + line
		}
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
}

// runAccessKeysListSaved lists the profile's keys, cross-checking the API
// when it is reachable.
func runAccessKeysListSaved(ctx context.Context, cmd *cobra.Command, filter listFilter) error {
	_, profileName, p, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return printErr(err)
	}
	keys := p.ObjectStorageKeys()
	names := p.SortedObjectStorageKeyNames()

	// Best-effort API cross-check over the projects the saved keys mention.
	known, queried := apiKeyIDs(ctx, cmd, keys, filter.Project)

	var rows []savedKeyRow
	for _, name := range names {
		k := keys[name]
		if !filter.matchSaved(k) {
			continue
		}
		row := newSavedKeyRow(name, k, profileName)
		// A key whose project was never listed cannot be judged: leaving the
		// column empty is honest, "missing on API" would not be.
		if known != nil && k.ProjectID != "" && queried[k.ProjectID] {
			if known[k.AccessKeyID] {
				row.API = "ok"
			} else {
				row.API = "missing on API"
			}
		}
		rows = append(rows, row)
	}
	if isHuman() {
		if len(rows) == 0 {
			objectstorage.Hintf("no access keys saved in profile %s; run 'lsh s3 configure' or 'lsh s3 access-keys create --save'", profileName)
			return nil
		}
		writeSavedKeyTable(os.Stdout, rows, known != nil)
		return nil
	}
	out := make([]renderer.ResponseData, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	render(out)
	return nil
}

// apiKeyIDs returns the set of access key IDs the API knows for the projects
// referenced by the saved keys (or the --project filter), or nil when the API
// could not be consulted (not logged in, network error, no project known).
func apiKeyIDs(ctx context.Context, cmd *cobra.Command, keys map[string]config.StoredAccessKey, project string) (known, queried map[string]bool) {
	if endpointOverride(cmd) != "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var projects []string
	if project != "" {
		projects = []string{project}
	} else {
		for _, k := range keys {
			if k.ProjectID != "" && !seen[k.ProjectID] {
				seen[k.ProjectID] = true
				projects = append(projects, k.ProjectID)
			}
		}
		sort.Strings(projects)
	}
	if len(projects) == 0 {
		return nil, nil
	}
	api := newKeysAPI()
	known = map[string]bool{}
	queried = map[string]bool{}
	for _, p := range projects {
		list, err := api.list(ctx, p)
		if err != nil {
			lsh.LogDebugf("[s3] could not cross-check saved keys with the API for project %s: %v", p, err)
			return nil, nil
		}
		queried[p] = true
		for _, k := range list {
			known[k.AccessKeyID] = true
		}
	}
	return known, queried
}

// writeSavedKeyTable prints the --saved listing.
func writeSavedKeyTable(w io.Writer, rows []savedKeyRow, withAPI bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	header := "NAME\tACCESS KEY ID\tCLASS\tSITE\tPROJECT\tSCOPE\tBUCKETS\tSOURCE\tCREATED"
	if withAPI {
		header += "\tAPI"
	}
	fmt.Fprintln(tw, header)
	for _, r := range rows {
		line := strings.Join([]string{r.Name, r.AccessKeyID, dash(r.StorageClass), dash(r.Site), dash(r.ProjectID), dash(r.Scope), r.bucketsCell(), dash(r.Source), dash(r.CreatedAt)}, "\t")
		if withAPI {
			line += "\t" + dash(r.API)
		}
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
}
