package s3

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/spf13/cobra"
)

// optAllProjects is --all-projects (same spelling as ls).
const optAllProjects = "all-projects"

// metricsConcurrency bounds the parallel GetStorageBucketMetrics calls.
const metricsConcurrency = 8

// MetricsRow is one bucket's consumption and estimated cost for the current
// billing period, as returned by GET /storage/buckets/{id}/metrics.
type MetricsRow struct {
	Bucket       string     `json:"bucket"`
	ID           string     `json:"id"`
	Project      string     `json:"project,omitempty"`
	StorageClass string     `json:"storage_class,omitempty"`
	Site         string     `json:"site,omitempty"`
	CurrentGB    *int64     `json:"current_gb"`
	ConsumedGB   *int64     `json:"consumed_gb"`
	Unit         string     `json:"unit,omitempty"`
	CostAmount   *float64   `json:"estimated_cost"`
	Currency     string     `json:"currency,omitempty"`
	PeriodStart  *time.Time `json:"period_start,omitempty"`
	PeriodEnd    *time.Time `json:"period_end,omitempty"`
	// Total marks the aggregated row appended in table mode.
	Total bool `json:"-"`
	// costs is the per-currency sum of a TOTAL row (several currencies may mix).
	costs map[string]float64
}

// newMetricsRow converts an API payload for bucket b.
func newMetricsRow(b *objectstorage.Bucket, site string, data *operations.GetStorageBucketMetricsData) MetricsRow {
	row := MetricsRow{Bucket: b.Name, ID: b.ID, Project: b.ProjectRef(), StorageClass: b.StorageClass, Site: site}
	if row.Site == "" {
		row.Site = b.Site
	}
	if data == nil || data.Attributes == nil {
		return row
	}
	a := data.Attributes
	if a.Storage != nil {
		row.CurrentGB = a.Storage.Current
		row.ConsumedGB = a.Storage.Consumed
		if a.Storage.Unit != nil {
			row.Unit = string(*a.Storage.Unit)
		}
	}
	if a.EstimatedCost != nil {
		row.CostAmount = a.EstimatedCost.Amount
		if a.EstimatedCost.Currency != nil {
			row.Currency = *a.EstimatedCost.Currency
		}
	}
	if a.Period != nil {
		row.PeriodStart = a.Period.Start
		row.PeriodEnd = a.Period.End
	}
	return row
}

func (m MetricsRow) TableRow() table.Row {
	return table.Row{
		"id":             {Label: "ID", Value: m.ID},
		"bucket":         {Label: "Bucket", Value: m.Bucket},
		"project":        {Label: "Project", Value: m.Project},
		"storage_class":  {Label: "Class", Value: m.StorageClass},
		"region":         {Label: "Site", Value: m.Site},
		"current_gb":     {Label: "Current (GB)", Value: gbLabel(m.CurrentGB)},
		"consumed_gb":    {Label: "Consumed (GB)", Value: gbLabel(m.ConsumedGB)},
		"estimated_cost": {Label: "Est. Cost", Value: m.costLabel()},
		"period_start":   {Label: "Period Start", Value: dateLabel(m.PeriodStart)},
		"period_end":     {Label: "Period End", Value: dateLabel(m.PeriodEnd)},
	}
}

// costLabel renders "12.30 USD" (or the per-currency sums of a TOTAL row).
func (m MetricsRow) costLabel() string {
	if m.Total && len(m.costs) > 0 {
		currencies := make([]string, 0, len(m.costs))
		for c := range m.costs {
			currencies = append(currencies, c)
		}
		sort.Strings(currencies)
		parts := make([]string, 0, len(currencies))
		for _, c := range currencies {
			parts = append(parts, formatCost(m.costs[c], c))
		}
		return strings.Join(parts, " + ")
	}
	if m.CostAmount == nil {
		return emptyCell
	}
	return formatCost(*m.CostAmount, m.Currency)
}

func formatCost(amount float64, currency string) string {
	return strings.TrimSpace(fmt.Sprintf("%.2f %s", amount, currency))
}

// gbLabel renders the GB figure, or the shared table placeholder when unset.
func gbLabel(v *int64) string {
	if v == nil {
		return emptyCell
	}
	return fmt.Sprintf("%d", *v)
}

func dateLabel(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	// The API reports billing-period boundaries in UTC; rendering them in the
	// local zone shifts them a day west of Greenwich.
	return t.UTC().Format("2006-01-02")
}

// costOf returns the estimated cost or 0 when unknown.
func (m MetricsRow) costOf() float64 {
	if m.CostAmount == nil {
		return 0
	}
	return *m.CostAmount
}

// sortMetricsRows orders rows by estimated cost (desc), then consumed GB
// (desc), then bucket name so the output is stable.
func sortMetricsRows(rows []MetricsRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ci, cj := rows[i].costOf(), rows[j].costOf()
		if ci != cj {
			return ci > cj
		}
		gi, gj := int64Of(rows[i].ConsumedGB), int64Of(rows[j].ConsumedGB)
		if gi != gj {
			return gi > gj
		}
		return rows[i].Bucket < rows[j].Bucket
	})
}

func int64Of(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// metricsTotal sums current/consumed GB and the cost per currency across rows.
// Period columns are filled when every row shares the same period.
func metricsTotal(rows []MetricsRow) MetricsRow {
	total := MetricsRow{Bucket: "TOTAL", Total: true, costs: map[string]float64{}}
	var current, consumed int64
	var haveCurrent, haveConsumed bool
	var currency string
	var samePeriod = true
	for i, r := range rows {
		if r.CurrentGB != nil {
			current += *r.CurrentGB
			haveCurrent = true
		}
		if r.ConsumedGB != nil {
			consumed += *r.ConsumedGB
			haveConsumed = true
		}
		if r.CostAmount != nil {
			total.costs[r.Currency] += *r.CostAmount
		}
		if r.Unit != "" && total.Unit == "" {
			total.Unit = r.Unit
		}
		if i == 0 {
			total.PeriodStart, total.PeriodEnd = r.PeriodStart, r.PeriodEnd
		} else if !sameTime(total.PeriodStart, r.PeriodStart) || !sameTime(total.PeriodEnd, r.PeriodEnd) {
			samePeriod = false
		}
	}
	if haveCurrent {
		total.CurrentGB = &current
	}
	if haveConsumed {
		total.ConsumedGB = &consumed
	}
	if len(total.costs) == 1 {
		for c, amount := range total.costs {
			currency = c
			a := amount
			total.CostAmount = &a
		}
		total.Currency = currency
	}
	if !samePeriod {
		total.PeriodStart, total.PeriodEnd = nil, nil
	}
	return total
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// metricsResult pairs a bucket with its metrics or the error fetching them.
type metricsResult struct {
	Bucket *objectstorage.Bucket
	Row    MetricsRow
	Err    error
}

// fetchMetrics calls GetStorageBucketMetrics for every bucket with bounded
// concurrency. Per-bucket failures do not stop the others; results keep the
// input order.
func fetchMetrics(ctx context.Context, api *sdk.Latitudesh, buckets []*objectstorage.Bucket, sites map[string]string, opts []operations.Option) []metricsResult {
	results := make([]metricsResult, len(buckets))
	sem := make(chan struct{}, metricsConcurrency)
	var wg sync.WaitGroup
	for i, b := range buckets {
		wg.Add(1)
		go func(i int, b *objectstorage.Bucket) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res := metricsResult{Bucket: b}
			resp, err := api.ObjectStorage.GetStorageBucketMetrics(ctx, b.ID, opts...)
			if err != nil {
				res.Err = objectstorage.HumanizeAPI(err, fmt.Sprintf("metrics for bucket %s", b.Display()))
			} else {
				var data *operations.GetStorageBucketMetricsData
				if resp.Object != nil {
					data = resp.Object.Data
				}
				res.Row = newMetricsRow(b, sites[b.ID], data)
			}
			results[i] = res
		}(i, b)
	}
	wg.Wait()
	return results
}

// NewMetricsCmd builds `lsh s3 metrics [s3://bucket]`.
func NewMetricsCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "metrics [s3://bucket]",
		GroupID: groupReports,
		Short:   "Current size and estimated cost of a bucket",
		Long: `Show the storage consumed in the current billing period and the estimated
cost, per bucket. Without a bucket every bucket of the project (or of the team
with --all-projects) is listed, sorted by estimated cost, with a TOTAL row in
table output. Structured output (-o json|yaml|csv) stays one record per bucket.`,
		Example: `  lsh s3 metrics s3://backups
  lsh s3 metrics --project my-project
  lsh s3 metrics --all-projects -o json --query '[].{bucket:bucket,cost:estimated_cost}'`,
		Args: cobra.MaximumNArgs(1),
		RunE: runMetrics,
	})
	addProjectFlag(cmd, true, "only buckets of this project (ID or slug)")
	addBucketFilterFlags(cmd)
	cmd.Flags().Bool(optAllProjects, false, "buckets of every project of the team")
	return cmd
}

func runMetrics(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	if endpointOverride(cmd) != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "metrics come from the Latitude API; unset --endpoint-url / %s to use this command", objectstorage.EnvEndpointURL))
	}
	allProjects, _ := cmd.Flags().GetBool(optAllProjects)
	if allProjects && cmd.Flags().Changed(flagProject) {
		return printErr(exitcode.Errorf(exitcode.Usage, "--%s and --%s are mutually exclusive", flagProject, optAllProjects))
	}

	var buckets []*objectstorage.Bucket
	var sites map[string]string
	if len(args) == 1 {
		ref, err := objectstorage.ParseBucketOnly(args[0])
		if err != nil {
			return printErr(err)
		}
		b, err := resolveBucket(ctx, cmd, ref.Bucket)
		if err != nil {
			return printErr(err)
		}
		if s, err := objectstorage.RawBucketSites(ctx, b.ID); err == nil {
			sites = s
		} else {
			lsh.LogDebugf("metrics: could not fetch site for %s: %v", b.ID, err)
		}
		buckets = []*objectstorage.Bucket{b}
	} else {
		r := newResolver(cmd)
		if allProjects {
			r.Project = ""
		}
		list, err := r.ListBuckets(ctx)
		if err != nil {
			return printErr(err)
		}
		// Scope the site lookup to the project when one is known; only
		// --all-projects needs the team-wide listing.
		if s, err := objectstorage.RawBucketSitesForProject(ctx, "", r.Project); err == nil {
			sites = s
		} else {
			lsh.LogDebugf("metrics: could not fetch bucket sites: %v", err)
		}
		// --storage-class/--site also narrow the multi-bucket listing, not just
		// a single ambiguous name.
		list = filterBucketsByClass(list, r.ClassFilter)
		for _, d := range list {
			b := objectstorage.BucketFromData(d)
			if r.SiteFilter != "" {
				site := b.Site
				if site == "" && d.ID != nil {
					site = sites[*d.ID]
				}
				if !strings.EqualFold(site, r.SiteFilter) {
					continue
				}
			}
			buckets = append(buckets, b)
		}
	}

	results := fetchMetrics(ctx, apiClient(), buckets, sites, []operations.Option{operations.WithRetries(lsh.RetryConfig())})
	rows := make([]MetricsRow, 0, len(results))
	var failed int
	var firstErr error
	for _, res := range results {
		if res.Err != nil {
			failed++
			if firstErr == nil {
				firstErr = res.Err
			}
			objectstorage.Warnf("metrics unavailable for %s: %v", res.Bucket.Display(), res.Err)
			continue
		}
		rows = append(rows, res.Row)
	}
	if failed > 0 && len(rows) == 0 {
		return printErr(firstErr)
	}

	sortMetricsRows(rows)
	if isHuman() && len(rows) > 1 {
		rows = append(rows, metricsTotal(rows))
	}
	render(objectstorage.AsResponseData(rows))
	if failed > 0 {
		return printErr(exitcode.Errorf(exitcode.Partial, "metrics unavailable for %d of %d buckets", failed, len(results)))
	}
	return nil
}
