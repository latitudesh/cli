package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

// Local flag names of `ls`.
const (
	flagRecursive     = "recursive"
	flagHumanReadable = "human-readable"
	flagSummarize     = "summarize"
	flagVersions      = "versions"
	flagStartingToken = "starting-token"
	flagStream        = "stream"
	flagAllProjects   = "all-projects"
	flagStorageClass  = "storage-class"
)

// maxListPageSize is the largest max-keys S3 accepts per request.
const maxListPageSize = 1000

// NewLsCmd builds `lsh s3 list [s3://bucket[/prefix]]`.
func NewLsCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "list [s3://bucket[/prefix]]",
		Aliases: []string{"ls"},
		GroupID: groupBuckets,
		Short:   "List buckets, or the objects under a prefix (alias: ls)",
		Long: `List buckets, or the objects under a prefix.

Without an argument, lists buckets through the Latitude API. Pick a project
with --project (or LSH_PROJECT), list every project with --all-projects, or —
in a terminal — choose from the project picker (which includes "All projects").
--storage-class and --site narrow the listing. With an
argument, lists the objects and common prefixes under s3://<bucket>[/<prefix>]
straight from the bucket's S3 endpoint: the prefix is
matched literally ('s3://b/2026' matches every key starting with "2026"),
sub-prefixes are shown as PRE entries unless --recursive is given, and an empty
listing prints nothing and exits 0.

Pagination follows the global flags: --page-size (default and maximum 1000
keys per request), --max-items to cap the total number of entries and
--no-paginate to stop after one page; the token to resume from is printed on
stderr and accepted by --starting-token.

Structured output (-o json|yaml|csv) emits one row per object or prefix;
--stream with -o json prints one JSON object per line as the listing arrives
instead of buffering everything.`,
		Example: `  lsh s3 list
  lsh s3 list --project my-project --storage-class high_performance
  lsh s3 list s3://backups/2026/09/
  lsh s3 list s3://backups --recursive --human-readable --summarize
  lsh s3 list s3://backups/logs/ --versions
  lsh s3 list s3://backups --recursive -o json --stream | jq -r .key
  lsh s3 list s3://backups --no-paginate --page-size 100
  lsh s3 list s3://backups --starting-token <token>`,
		Args: cobra.MaximumNArgs(1),
		RunE: runLs,
	})

	addProjectFlag(cmd, true, "only list buckets of this project (ID or slug)")
	cmd.Flags().Bool(flagAllProjects, false, "list buckets across every project (skip the project picker)")
	cmd.Flags().StringP(flagStorageClass, "c", "", "only list buckets of this storage class (standard or high_performance)")
	cmd.Flags().String(flagSite, "", "only list buckets in this Latitude site (e.g. DAL, TYO4)")
	cmd.Flags().BoolP(flagRecursive, "r", false, "list every object under the prefix instead of stopping at the next '/'")
	cmd.Flags().BoolP(flagHumanReadable, "H", false, "print sizes in human readable units (KiB, MiB…)")
	cmd.Flags().Bool(flagSummarize, false, "append the total number of objects and their size")
	cmd.Flags().Bool(flagVersions, false, "list every object version (versioned buckets)")
	cmd.Flags().String(flagStartingToken, "", "resume the listing from the token printed by a previous --no-paginate or --max-items run")
	cmd.Flags().Bool(flagStream, false, "with -o json, print one JSON object per line as the listing arrives (NDJSON)")
	unsupportedAWSBoolFlags(cmd, map[string]string{
		"request-payer": "there is no requester-pays billing on Latitude object storage",
	})
	rejectRegionFlag(cmd)

	return cmd
}

func runLs(cmd *cobra.Command, args []string) error {
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	if len(args) == 0 {
		return runLsBuckets(ctx, cmd)
	}
	return runLsObjects(ctx, cmd, args[0])
}

// ---------------------------------------------------------------------------
// Buckets
// ---------------------------------------------------------------------------

func runLsBuckets(ctx context.Context, cmd *cobra.Command) error {
	if endpointOverride(cmd) != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "listing buckets needs the Latitude API and cannot be combined with --endpoint-url; pass s3://<bucket> to list objects on that endpoint"))
	}
	classFlag, _ := cmd.Flags().GetString(flagStorageClass)
	// Shared alias table (standard|std|wasabi, high_performance|high-performance|
	// hp|high|vast); an empty value means "no filter".
	class, err := objectstorage.ParseStorageClass(classFlag)
	if err != nil {
		return printErr(fmt.Errorf("--%s: %w", flagStorageClass, err))
	}

	site, _ := cmd.Flags().GetString(flagSite)

	// Pick the project the same way as the rest of the CLI: --project /
	// LSH_PROJECT, or --all-projects, otherwise an interactive picker with an
	// "All projects" entry (and a usage error in a non-interactive session).
	project, _, err := cli.PickProjectForList(cmd)
	if err != nil {
		return printErr(err)
	}

	r := newResolver(cmd)
	r.Project = project // "" when the user chose all projects
	data, err := r.ListBuckets(ctx)
	if err != nil {
		return printErr(objectstorage.Humanize(err, nil, nil))
	}
	data = filterBucketsByClass(data, class)

	// The SDK model drops the site slug, so fetch it best-effort for the Site
	// column (and for the --site filter), scoped to the same --project filter
	// as the listing.
	sites, siteErr := objectstorage.RawBucketSitesForProject(ctx, "", project)
	if siteErr != nil {
		sites = nil
	}
	if s := strings.TrimSpace(site); s != "" {
		// The filter depends entirely on that lookup (the SDK model drops the
		// slug), so a failure would silently drop every bucket and exit 0.
		if siteErr != nil {
			return printErr(exitcode.Errorf(exitcode.Generic, "--site %s needs the sites of the listed buckets, which could not be fetched: %v", s, siteErr))
		}
		data = filterBucketsBySite(data, s, sites)
	}

	// Render through the shared renderer, exactly like every other `list`
	// command (servers, volume, filesystems…): the default table, or
	// -o json|yaml|csv|text with --query.
	render(BucketRows(data, sites))
	return nil
}

// filterBucketsBySite keeps the buckets in one Latitude site ("" keeps all).
// The site slug comes from the SDK model when present, otherwise from the
// site map fetched alongside the listing.
func filterBucketsBySite(data []components.ObjectStorageData, site string, sites map[string]string) []components.ObjectStorageData {
	out := make([]components.ObjectStorageData, 0, len(data))
	for _, d := range data {
		s := objectstorage.BucketFromData(d).Site
		if s == "" && d.ID != nil {
			s = sites[*d.ID]
		}
		if strings.EqualFold(s, site) {
			out = append(out, d)
		}
	}
	return out
}

// filterBucketsByClass keeps the buckets of one storage class ("" keeps all).
func filterBucketsByClass(data []components.ObjectStorageData, class string) []components.ObjectStorageData {
	if class == "" {
		return data
	}
	out := make([]components.ObjectStorageData, 0, len(data))
	for _, d := range data {
		if strings.EqualFold(objectstorage.BucketFromData(d).StorageClass, class) {
			out = append(out, d)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Objects
// ---------------------------------------------------------------------------

// listOptions controls listObjects.
type listOptions struct {
	// Prefix is matched literally (no trailing slash is added).
	Prefix string
	// Recursive drops the "/" delimiter.
	Recursive bool
	// Versions lists object versions instead of current objects.
	Versions bool
	// PageSize is the max-keys sent per request (1..1000; 0 means 1000).
	PageSize int
	// MaxItems caps the total number of entries (0 = unlimited).
	MaxItems int64
	// NoPaginate stops after the first page.
	NoPaginate bool
	// StartingToken resumes a ListObjectsV2 listing from a continuation token.
	StartingToken string
	// OnEntry, when set, receives every entry as it arrives instead of it
	// being accumulated in listResult.Entries (streaming output).
	OnEntry func(renderer.ResponseData)
}

// listResult is the outcome of listObjects.
type listResult struct {
	// Entries holds prefixes and objects in listing order (empty when
	// listOptions.OnEntry streamed them).
	Entries []renderer.ResponseData
	// Count and Bytes summarize the listed objects (prefixes excluded).
	Count int
	Bytes int64
	// Total is the number of entries emitted (prefixes included).
	Total int64
	// NextToken is the continuation token to resume from when the listing
	// stopped before the end ("" when exhausted).
	NextToken string
}

func (r *listResult) emit(opts listOptions, e renderer.ResponseData) {
	r.Total++
	if o, ok := e.(objectstorage.Object); ok {
		r.Count++
		r.Bytes += o.Size
	}
	if opts.OnEntry != nil {
		opts.OnEntry(e)
		return
	}
	r.Entries = append(r.Entries, e)
}

// listObjects lists the objects (or versions) of bucketName under the prefix,
// paginating with continuation tokens and honoring the page-size, max-items,
// no-paginate and starting-token controls. It only issues GET requests.
func listObjects(ctx context.Context, client *minio.Client, bucketName string, opts listOptions) (listResult, error) {
	if opts.PageSize <= 0 || opts.PageSize > maxListPageSize {
		opts.PageSize = maxListPageSize
	}
	if opts.Versions {
		return listVersions(ctx, client, bucketName, opts)
	}

	var res listResult
	delimiter := "/"
	if opts.Recursive {
		delimiter = ""
	}
	core := minio.Core{Client: client}
	token := opts.StartingToken
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		maxKeys := opts.PageSize
		if opts.MaxItems > 0 {
			remaining := opts.MaxItems - res.Total
			if remaining <= 0 {
				return res, nil
			}
			if remaining < int64(maxKeys) {
				maxKeys = int(remaining)
			}
		}
		page, err := listPage(ctx, core, bucketName, opts.Prefix, token, delimiter, maxKeys)
		if err != nil {
			return res, err
		}
		for _, cp := range page.CommonPrefixes {
			res.emit(opts, objectstorage.NewPrefix(cp.Prefix))
		}
		for _, info := range page.Contents {
			res.emit(opts, objectstorage.ObjectFromInfo(info, false))
		}
		next := page.NextContinuationToken
		if !page.IsTruncated || next == "" {
			res.NextToken = ""
			return res, nil
		}
		res.NextToken = next
		if opts.NoPaginate {
			return res, nil
		}
		if opts.MaxItems > 0 && res.Total >= opts.MaxItems {
			return res, nil
		}
		token = next
	}
}

// listPage issues one ListObjectsV2 request. minio.Core.ListObjectsV2 takes no
// context (it is the only entry point that exposes continuation tokens, which
// --starting-token / --no-paginate need), so the call runs in a goroutine and
// the caller stops waiting as soon as ctx is cancelled: Ctrl-C then returns
// ctx.Err(), which Humanize maps to exit 130. An abandoned request finishes in
// the background and its result is dropped (the channel is buffered).
func listPage(ctx context.Context, core minio.Core, bucketName, prefix, token, delimiter string, maxKeys int) (minio.ListBucketV2Result, error) {
	type pageResult struct {
		page minio.ListBucketV2Result
		err  error
	}
	done := make(chan pageResult, 1)
	go func() {
		page, err := core.ListObjectsV2(bucketName, prefix, "", token, delimiter, maxKeys)
		done <- pageResult{page: page, err: err}
	}()
	select {
	case <-ctx.Done():
		return minio.ListBucketV2Result{}, ctx.Err()
	case r := <-done:
		return r.page, r.err
	}
}

// listVersions lists object versions (ListObjectVersions) through the minio
// iterator. Versions listings have no continuation token to expose, so
// --no-paginate and --starting-token do not apply; --max-items does.
func listVersions(ctx context.Context, client *minio.Client, bucketName string, opts listOptions) (listResult, error) {
	var res listResult
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:       opts.Prefix,
		Recursive:    opts.Recursive,
		WithVersions: true,
		MaxKeys:      opts.PageSize,
	})
	for info := range ch {
		if info.Err != nil {
			return res, info.Err
		}
		if opts.MaxItems > 0 && res.Total >= opts.MaxItems {
			cancel()
			// Drain so the producer goroutine exits.
			for range ch {
			}
			break
		}
		// minio reports common prefixes of a non-recursive listing as entries
		// with a key ending in "/" and no version.
		if !opts.Recursive && strings.HasSuffix(info.Key, "/") && info.VersionID == "" && info.Size == 0 {
			res.emit(opts, objectstorage.NewPrefix(info.Key))
			continue
		}
		res.emit(opts, objectstorage.ObjectFromInfo(info, true))
	}
	return res, nil
}

// streamJSON writes one entry as a single JSON line (NDJSON).
func streamJSON(w io.Writer, e renderer.ResponseData) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// resolvePageSize returns the max-keys per request: --page-size when given
// explicitly (capped at 1000), else 1000.
func resolvePageSize(cmd *cobra.Command, resolved pagination.Options) int {
	if !cmd.Flags().Changed("page-size") {
		return maxListPageSize
	}
	if resolved.PageSize <= 0 || resolved.PageSize > maxListPageSize {
		return maxListPageSize
	}
	return int(resolved.PageSize)
}

func runLsObjects(ctx context.Context, cmd *cobra.Command, arg string) error {
	ref, err := objectstorage.ParseRemote(arg)
	if err != nil {
		return printErr(err)
	}
	recursive, _ := cmd.Flags().GetBool(flagRecursive)
	human, _ := cmd.Flags().GetBool(flagHumanReadable)
	summarize, _ := cmd.Flags().GetBool(flagSummarize)
	versions, _ := cmd.Flags().GetBool(flagVersions)
	stream, _ := cmd.Flags().GetBool(flagStream)
	startingToken, _ := cmd.Flags().GetString(flagStartingToken)

	pg := pagination.Resolve()
	opts := listOptions{
		Prefix:        ref.Key,
		Recursive:     recursive,
		Versions:      versions,
		PageSize:      resolvePageSize(cmd, pg),
		MaxItems:      pg.MaxItems,
		NoPaginate:    pg.NoPaginate,
		StartingToken: startingToken,
	}
	if versions && (pg.NoPaginate || startingToken != "") {
		objectstorage.Warnf("--no-paginate and --starting-token do not apply to --versions listings; ignoring them")
		opts.NoPaginate = false
		opts.StartingToken = ""
	}

	format := renderer.ResolveFormat()
	if stream && format != renderer.FormatJSON {
		objectstorage.Warnf("--stream only applies to -o json; ignoring it")
		stream = false
	}

	if dryRun() {
		objectstorage.Hintf("(dryrun) ls is read-only; listing %s", ref)
	}

	sess, err := openBucket(ctx, cmd, ref.Bucket, false)
	if err != nil {
		return printErr(objectstorage.Humanize(err, nil, nil))
	}

	// Objects render through the shared renderer like every other list command
	// (default table, or -o json|yaml|csv|text with --query). --stream keeps
	// the NDJSON fast path for -o json over very large listings.
	if stream {
		opts.OnEntry = func(e renderer.ResponseData) {
			if err := streamJSON(os.Stdout, e); err != nil {
				objectstorage.Warnf("could not write entry: %v", err)
			}
		}
	}

	res, err := listObjects(ctx, sess.Client, sess.Bucket.BucketName, opts)
	if err != nil {
		return printErr(sess.humanize(err))
	}

	if !stream {
		render(res.Entries)
		if summarize && renderer.ResolveFormat() == renderer.FormatTable {
			fmt.Print(objectstorage.LsSummary(res.Count, res.Bytes, human))
			fmt.Println()
		}
	}

	if res.NextToken != "" {
		objectstorage.Hintf("Next token: %s (use --starting-token)", res.NextToken)
	}
	return nil
}
