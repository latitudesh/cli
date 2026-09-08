package s3

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/latitudesh-go-sdk/types"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
)

const (
	flagSince        = "since"
	flagUntil        = "until"
	flagUsageBucket  = "bucket"
	flagGroupBy      = "group-by"
	optHumanReadable = "human-readable"

	defaultUsageSince = "30d"

	// usageNameLookupMax caps the per-bucket calls used to map the API's
	// numeric storage_id to bucket names; above it the raw ids are shown.
	usageNameLookupMax = 50
	// usageLookupConcurrency bounds those parallel calls.
	usageLookupConcurrency = metricsConcurrency
)

// Usage groupings.
const (
	groupByDay    = "day"
	groupByBucket = "bucket"
	groupByTier   = "tier"
	groupByRegion = "region"
)

var usageGroupings = []string{groupByDay, groupByBucket, groupByTier, groupByRegion}

// usageSample is one daily usage record after name resolution.
type usageSample struct {
	Date      string // YYYY-MM-DD
	Project   string
	StorageID string // the API's numeric storage id ("2028"), not the bkt_ id
	Bucket    string // display name when known, else StorageID
	Named     bool   // Bucket is a resolved display name
	Tier      string
	Region    string
	Bytes     int64
}

// UsageRow is an aggregated usage record (one per group key).
type UsageRow struct {
	Date      string `json:"date,omitempty"`
	Project   string `json:"project,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	StorageID string `json:"storage_id,omitempty"`
	Tier      string `json:"tier,omitempty"`
	Region    string `json:"region,omitempty"`
	// Bytes is the sum of the daily samples in the group (byte-days when the
	// group spans several days).
	Bytes int64 `json:"bytes"`
	// Days is the number of distinct days in the group; AvgBytes = Bytes/Days.
	Days     int    `json:"days,omitempty"`
	AvgBytes int64  `json:"avg_bytes,omitempty"`
	Size     string `json:"size,omitempty"`     // human size of Bytes (-H only)
	AvgSize  string `json:"avg_size,omitempty"` // human size of AvgBytes (-H only)

	groupBy string
	human   bool
	// rawIDs is set when no bucket name could be resolved (too many buckets
	// for the lookup, or every bucket gone): the table then shows the API's
	// storage_id under "Storage ID" instead of a "Bucket" column.
	rawIDs bool
}

func (u UsageRow) TableRow() table.Row {
	bytesCell := fmt.Sprintf("%d", u.Bytes)
	avgCell := fmt.Sprintf("%d", u.AvgBytes)
	if u.human {
		bytesCell = objectstorage.HumanSize(u.Bytes)
		avgCell = objectstorage.HumanSize(u.AvgBytes)
	}
	row := table.Row{
		"project": {Label: "Project", Value: u.Project},
		"tier":    {Label: "Tier", Value: u.Tier},
		"region":  {Label: "Region", Value: u.Region},
		"bytes":   {Label: "Bytes", Value: bytesCell},
	}
	if u.rawIDs {
		row["storage_id"] = table.Cell{Label: "Storage ID", Value: u.StorageID}
	} else {
		row["bucket"] = table.Cell{Label: "Bucket", Value: u.Bucket}
	}
	if u.groupBy == groupByDay || u.groupBy == "" {
		row["date"] = table.Cell{Label: "Date", Value: u.Date}
	} else {
		row["days"] = table.Cell{Label: "Days", Value: fmt.Sprintf("%d", u.Days)}
		row["avg_bytes"] = table.Cell{Label: "Avg/Day", Value: avgCell}
	}
	return row
}

// parseUsageWindow resolves --since/--until (utils.ParseTimeRef syntax) with
// the defaults since=30d and until=now, and rejects an inverted window.
func parseUsageWindow(since, until string, now time.Time) (time.Time, time.Time, error) {
	since = strings.TrimSpace(since)
	if since == "" {
		since = defaultUsageSince
	}
	start, err := utils.ParseTimeRef(since, now)
	if err != nil {
		return time.Time{}, time.Time{}, exitcode.Errorf(exitcode.Usage, "--%s: %v", flagSince, err)
	}
	end := now
	if u := strings.TrimSpace(until); u != "" && !strings.EqualFold(u, "now") {
		end, err = utils.ParseTimeRef(u, now)
		if err != nil {
			return time.Time{}, time.Time{}, exitcode.Errorf(exitcode.Usage, "--%s: %v", flagUntil, err)
		}
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, exitcode.Errorf(exitcode.Usage, "--%s (%s) is after --%s (%s)", flagSince, start.Format("2006-01-02"), flagUntil, end.Format("2006-01-02"))
	}
	return start, end, nil
}

// validateGroupBy checks the --group-by value.
func validateGroupBy(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return groupByDay, nil
	}
	for _, g := range usageGroupings {
		if v == g {
			return g, nil
		}
	}
	return "", exitcode.Errorf(exitcode.Usage, "invalid --%s %q: expected one of %s", flagGroupBy, v, strings.Join(usageGroupings, ", "))
}

// usageSamples converts API rows for one project. names maps the API's
// numeric storage ids to display names (see usageStorageIDNames);
// fallbackBucket is used when the request was filtered to one bucket and the
// id is unknown.
func usageSamples(data []components.StorageUsageData, project string, names map[string]string, fallbackBucket string) []usageSample {
	out := make([]usageSample, 0, len(data))
	for _, d := range data {
		a := d.Attributes
		if a == nil {
			continue
		}
		// The endpoint has no storage_type filter and reports object, file and
		// block rows; summing them would inflate object storage usage.
		if t := strings.ToLower(utils.Str(a.StorageType)); t != "" && t != "object" {
			continue
		}
		s := usageSample{Project: project, StorageID: utils.Str(a.StorageID), Tier: utils.Str(a.Tier), Region: utils.Str(a.Region)}
		if a.Date != nil {
			s.Date = a.Date.String()
		}
		if a.Bytes != nil {
			s.Bytes = *a.Bytes
		}
		s.Bucket = s.StorageID
		if n, ok := names[s.StorageID]; ok && n != "" {
			s.Bucket, s.Named = n, true
		} else if fallbackBucket != "" {
			s.Bucket, s.Named = fallbackBucket, true
		}
		out = append(out, s)
	}
	return out
}

// aggregateUsage sums bytes per group. Every grouping keeps the project in
// the key so --all-projects stays readable; dimensions that are not part of
// the key are shown when uniform inside the group and left empty otherwise.
func aggregateUsage(samples []usageSample, groupBy string, human bool) []UsageRow {
	type acc struct {
		row    UsageRow
		days   map[string]struct{}
		bucket map[string]struct{}
		sid    map[string]struct{}
		tier   map[string]struct{}
		region map[string]struct{}
	}
	groups := map[string]*acc{}
	var order []string
	var named bool
	for _, s := range samples {
		named = named || s.Named
		var key string
		switch groupBy {
		case groupByBucket:
			key = s.Project + "\x00" + s.StorageID
		case groupByTier:
			key = s.Project + "\x00" + s.Tier
		case groupByRegion:
			key = s.Project + "\x00" + s.Region
		default:
			key = s.Project + "\x00" + s.Date
		}
		g, ok := groups[key]
		if !ok {
			g = &acc{
				row:    UsageRow{Project: s.Project, groupBy: groupBy, human: human},
				days:   map[string]struct{}{},
				bucket: map[string]struct{}{},
				sid:    map[string]struct{}{},
				tier:   map[string]struct{}{},
				region: map[string]struct{}{},
			}
			groups[key] = g
			order = append(order, key)
		}
		g.row.Bytes += s.Bytes
		g.days[s.Date] = struct{}{}
		g.bucket[s.Bucket] = struct{}{}
		g.sid[s.StorageID] = struct{}{}
		g.tier[s.Tier] = struct{}{}
		g.region[s.Region] = struct{}{}
	}

	rows := make([]UsageRow, 0, len(groups))
	for _, key := range order {
		g := groups[key]
		r := g.row
		r.Days = len(g.days)
		if groupBy == groupByDay {
			r.Date = uniform(g.days)
		}
		r.Bucket = uniform(g.bucket)
		r.StorageID = uniform(g.sid)
		r.Tier = uniform(g.tier)
		r.Region = uniform(g.region)
		if r.Days > 0 {
			r.AvgBytes = r.Bytes / int64(r.Days)
		}
		if human {
			r.Size = objectstorage.HumanSize(r.Bytes)
			r.AvgSize = objectstorage.HumanSize(r.AvgBytes)
		}
		if groupBy == groupByDay {
			// Per-day rows: Days is always 1 and the average equals Bytes.
			r.Days, r.AvgBytes, r.AvgSize = 0, 0, ""
		}
		r.rawIDs = !named
		rows = append(rows, r)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if groupBy == groupByDay {
			if rows[i].Date != rows[j].Date {
				return rows[i].Date < rows[j].Date
			}
			return rows[i].Project < rows[j].Project
		}
		if rows[i].Bytes != rows[j].Bytes {
			return rows[i].Bytes > rows[j].Bytes
		}
		if rows[i].Project != rows[j].Project {
			return rows[i].Project < rows[j].Project
		}
		return rows[i].Bucket+rows[i].Tier+rows[i].Region < rows[j].Bucket+rows[j].Tier+rows[j].Region
	})
	return rows
}

// uniform returns the single value of set, or "" when it has several.
func uniform(set map[string]struct{}) string {
	if len(set) != 1 {
		return ""
	}
	for v := range set {
		return v
	}
	return ""
}

// usageStorageIDNames maps the numeric storage_id carried by usage rows to
// bucket display names. The API exposes that id only through the usage rows
// themselves, but filter[storage_id] accepts the bkt_ id, so the window is
// queried once per bucket and the storage_id seen in the answer is recorded
// for that bucket. Calls run with bounded concurrency; a bucket whose call
// fails or returns no rows simply stays unmapped (its rows, if any, show the
// raw id). project is used for buckets that do not carry their own.
func usageStorageIDNames(ctx context.Context, api *sdk.Latitudesh, buckets []*objectstorage.Bucket, project string, start, end *types.Date, opts []operations.Option) map[string]string {
	names := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, usageLookupConcurrency)
	for _, b := range buckets {
		if b.ID == "" || b.Name == "" {
			continue
		}
		wg.Add(1)
		go func(b *objectstorage.Bucket) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := b.ProjectRef()
			if p == "" {
				p = project
			}
			id := b.ID
			resp, err := api.ObjectStorage.GetStorageUsage(ctx, p, &id, start, end, opts...)
			if err != nil {
				lsh.LogDebugf("usage: could not map storage id of %s: %v", b.Display(), err)
				return
			}
			if resp.StorageUsage == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, d := range resp.StorageUsage.Data {
				if d.Attributes == nil {
					continue
				}
				if sid := utils.Str(d.Attributes.StorageID); sid != "" {
					names[sid] = b.Name
				}
			}
		}(b)
	}
	wg.Wait()
	return names
}

// NewUsageCmd builds `lsh s3 usage`.
func NewUsageCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:   "usage",
		Short: "Daily storage usage history",
		Long: `Show the daily storage usage of a project (bytes stored per day), optionally
restricted to one bucket and aggregated per bucket, tier or region.

--project is required unless --all-projects or --bucket is given. Dates accept
durations (30d, 2w, 24h) or ISO dates (2026-06-01). Aggregated rows report the
sum of the daily samples (bytes), the number of days and the average per day.`,
		Example: `  lsh s3 usage --project my-project
  lsh s3 usage --project my-project --since 7d --group-by bucket --human-readable
  lsh s3 usage --bucket s3://backups --since 2026-08-01 --until 2026-08-31 -o csv
  lsh s3 usage --all-projects --group-by region -o json`,
		Args: cobra.NoArgs,
		RunE: runUsage,
	})
	// Registered as optional so the root pre-run never prompts; the command
	// enforces the --project / --all-projects / --bucket requirement itself.
	addProjectFlag(cmd, true, "project to report (ID or slug); required unless --all-projects or --bucket")
	cmd.Flags().Bool(optAllProjects, false, "report every project that has buckets")
	cmd.Flags().String(flagSince, defaultUsageSince, "start of the window: duration (30d, 2w) or date (2026-06-01)")
	cmd.Flags().String(flagUntil, "", "end of the window: duration or date (default: now)")
	cmd.Flags().String(flagUsageBucket, "", "only this bucket (s3://<bucket>, name or bkt_ id)")
	cmd.Flags().String(flagGroupBy, groupByDay, "aggregate rows by day, bucket, tier or region")
	cmd.Flags().BoolP(optHumanReadable, "H", false, "show sizes in human-readable units")
	return cmd
}

func runUsage(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	if endpointOverride(cmd) != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "usage comes from the Latitude API; unset --endpoint-url / %s to use this command", objectstorage.EnvEndpointURL))
	}
	sinceFlag, _ := cmd.Flags().GetString(flagSince)
	untilFlag, _ := cmd.Flags().GetString(flagUntil)
	start, end, err := parseUsageWindow(sinceFlag, untilFlag, time.Now())
	if err != nil {
		return printErr(err)
	}
	groupFlag, _ := cmd.Flags().GetString(flagGroupBy)
	groupBy, err := validateGroupBy(groupFlag)
	if err != nil {
		return printErr(err)
	}
	human, _ := cmd.Flags().GetBool(optHumanReadable)
	allProjects, _ := cmd.Flags().GetBool(optAllProjects)
	if allProjects && cmd.Flags().Changed(flagProject) {
		return printErr(exitcode.Errorf(exitcode.Usage, "--%s and --%s are mutually exclusive", flagProject, optAllProjects))
	}
	bucketFlag, _ := cmd.Flags().GetString(flagUsageBucket)
	project := projectFlag(cmd)
	if allProjects {
		project = ""
	}

	// Optional bucket filter; it can also supply the project.
	var storageID *string
	var bucketName string
	if bucketFlag != "" {
		ref, err := objectstorage.ParseBucketOnly(bucketFlag)
		if err != nil {
			return printErr(err)
		}
		b, err := resolveBucket(ctx, cmd, ref.Bucket)
		if err != nil {
			return printErr(err)
		}
		if b.ID == "" {
			return printErr(exitcode.Errorf(exitcode.Usage, "bucket %s has no API id; usage needs the Latitude API", b.Display()))
		}
		id := b.ID
		storageID = &id
		bucketName = b.Name
		if project == "" && !allProjects {
			project = b.ProjectRef()
		}
	}
	if project == "" && !allProjects {
		return printErr(exitcode.Errorf(exitcode.Usage, "--%s is required (pass --%s=<id|slug>, --%s, --%s <s3://bucket> or set LSH_PROJECT)", flagProject, flagProject, optAllProjects, flagUsageBucket))
	}

	// Bucket list: the buckets whose storage ids get a name and, with
	// --all-projects, the set of projects to query.
	r := newResolver(cmd)
	r.Project = project
	list, err := r.ListBuckets(ctx)
	if err != nil {
		return printErr(err)
	}
	buckets := make([]*objectstorage.Bucket, 0, len(list))
	projectSet := map[string]struct{}{}
	for _, d := range list {
		b := objectstorage.BucketFromData(d)
		buckets = append(buckets, b)
		if p := b.ProjectRef(); p != "" {
			projectSet[p] = struct{}{}
		}
	}
	projects := []string{project}
	if allProjects {
		projects = projects[:0]
		for p := range projectSet {
			projects = append(projects, p)
		}
		sort.Strings(projects)
		if len(projects) == 0 {
			objectstorage.Hintf("no buckets found in any project; nothing to report")
			render(nil)
			return nil
		}
	}

	api := apiClient()
	opts := []operations.Option{operations.WithRetries(lsh.RetryConfig())}
	startDate, endDate := types.NewDate(start), types.NewDate(end)

	// The rows carry a numeric storage_id, not the bkt_ id, so names come from
	// one filtered call per bucket. Skipped with --bucket (the filtered rows
	// all belong to that bucket) and for very large bucket sets, where the
	// table falls back to a Storage ID column.
	var names map[string]string
	switch {
	case storageID != nil:
	case len(buckets) > usageNameLookupMax:
		objectstorage.Hintf("more than %d buckets; showing storage ids instead of bucket names", usageNameLookupMax)
	default:
		names = usageStorageIDNames(ctx, api, buckets, project, startDate, endDate, opts)
	}

	var samples []usageSample
	for _, p := range projects {
		resp, err := api.ObjectStorage.GetStorageUsage(ctx, p, storageID, startDate, endDate, opts...)
		if err != nil {
			return printErr(objectstorage.HumanizeAPI(err, fmt.Sprintf("usage for project %q", p)))
		}
		if resp.StorageUsage == nil {
			continue
		}
		samples = append(samples, usageSamples(resp.StorageUsage.Data, p, names, bucketName)...)
	}
	if len(samples) == 0 && isHuman() {
		objectstorage.Hintf("no usage recorded between %s and %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	render(objectstorage.AsResponseData(aggregateUsage(samples, groupBy, human)))
	return nil
}
