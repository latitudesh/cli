package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

func ptrF(f float64) *float64 { return &f }

func metricsFixture() []MetricsRow {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 23, 59, 59, 0, time.UTC)
	return []MetricsRow{
		{Bucket: "cheap", ID: "bkt_1", CurrentGB: ptrI(10), ConsumedGB: ptrI(12), Unit: "GB", CostAmount: ptrF(1.5), Currency: "USD", PeriodStart: &start, PeriodEnd: &end},
		{Bucket: "big", ID: "bkt_2", CurrentGB: ptrI(500), ConsumedGB: ptrI(480), Unit: "GB", CostAmount: ptrF(30), Currency: "USD", PeriodStart: &start, PeriodEnd: &end},
		{Bucket: "free", ID: "bkt_3", CurrentGB: ptrI(0), ConsumedGB: ptrI(0), Unit: "GB", CostAmount: ptrF(0), Currency: "USD", PeriodStart: &start, PeriodEnd: &end},
		{Bucket: "brl", ID: "bkt_4", CurrentGB: ptrI(100), ConsumedGB: ptrI(90), Unit: "GB", CostAmount: ptrF(30), Currency: "BRL", PeriodStart: &start, PeriodEnd: &end},
		{Bucket: "unknown", ID: "bkt_5"}, // metrics payload without attributes
	}
}

func TestSortMetricsRows(t *testing.T) {
	rows := metricsFixture()
	sortMetricsRows(rows)
	var got []string
	for _, r := range rows {
		got = append(got, r.Bucket)
	}
	// Cost desc; ties (30 USD vs 30 BRL) by consumed desc; zero-cost rows by
	// consumed, then name.
	want := []string{"big", "brl", "cheap", "free", "unknown"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestMetricsTotal(t *testing.T) {
	rows := metricsFixture()
	total := metricsTotal(rows)
	if total.Bucket != "TOTAL" || !total.Total {
		t.Errorf("total row not marked: %+v", total)
	}
	if total.CurrentGB == nil || *total.CurrentGB != 610 {
		t.Errorf("current_gb = %v, want 610", total.CurrentGB)
	}
	if total.ConsumedGB == nil || *total.ConsumedGB != 582 {
		t.Errorf("consumed_gb = %v, want 582", total.ConsumedGB)
	}
	if total.costs["USD"] != 31.5 || total.costs["BRL"] != 30 {
		t.Errorf("costs = %v", total.costs)
	}
	// Mixed currencies: no single amount, label lists both (sorted).
	if total.CostAmount != nil {
		t.Errorf("mixed currencies must not collapse into one amount: %v", *total.CostAmount)
	}
	if got := total.TableRow()["estimated_cost"].Value; got != "30.00 BRL + 31.50 USD" {
		t.Errorf("cost label = %q", got)
	}
	// Same period on every row (rows without a period do not count).
	if total.PeriodStart != nil && total.TableRow()["period_start"].Value == "" {
		t.Errorf("period should be kept when uniform")
	}

	// Single currency: amount and currency are filled.
	single := metricsTotal(rows[:3])
	if single.CostAmount == nil || *single.CostAmount != 31.5 || single.Currency != "USD" {
		t.Errorf("single-currency total = %+v", single)
	}
	if got := single.TableRow()["estimated_cost"].Value; got != "31.50 USD" {
		t.Errorf("single-currency label = %q", got)
	}
	if single.TableRow()["period_start"].Value == "" || single.TableRow()["period_end"].Value == "" {
		t.Errorf("uniform period should be shown on the TOTAL row")
	}

	// Rows with no metrics at all yield a total without numbers.
	empty := metricsTotal([]MetricsRow{{Bucket: "x"}, {Bucket: "y"}})
	if empty.CurrentGB != nil || empty.ConsumedGB != nil || empty.CostAmount != nil {
		t.Errorf("empty total should have nil numbers: %+v", empty)
	}
	// Empty numbers render the shared "-" placeholder used by every s3 table.
	if got := empty.TableRow()["estimated_cost"].Value; got != emptyCell {
		t.Errorf("empty cost label = %q, want %q", got, emptyCell)
	}
	if got := empty.TableRow()["current_gb"].Value; got != emptyCell {
		t.Errorf("empty current_gb label = %q, want %q", got, emptyCell)
	}
}

func TestMetricsRowJSONShape(t *testing.T) {
	rows := metricsFixture()
	raw, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"bucket":"cheap"`, `"id":"bkt_1"`, `"current_gb":10`, `"consumed_gb":12`, `"estimated_cost":1.5`, `"currency":"USD"`, `"period_start":`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("json %s lacks %s", raw, want)
		}
	}
	// The TOTAL marker never leaks into structured output.
	total := metricsTotal(rows)
	raw, _ = json.Marshal(total)
	if strings.Contains(string(raw), "Total") || strings.Contains(string(raw), "costs") {
		t.Errorf("total internals leaked: %s", raw)
	}
}

// fakeMetricsAPI serves GET /storage/buckets/{id}/metrics; bkt_err fails
// with 500 and bkt_missing with 404. It records the peak concurrency.
func fakeMetricsAPI(t *testing.T, peak *int32) *httptest.Server {
	var inflight int32
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		defer atomic.AddInt32(&inflight, -1)
		mu.Lock()
		if cur > *peak {
			*peak = cur
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)

		if r.Header.Get("Authorization") == "" {
			t.Errorf("missing Authorization header")
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "storage" || parts[1] != "buckets" || parts[3] != "metrics" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		id := parts[2]
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch id {
		case "bkt_err":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"status":"500","title":"Internal Server Error","detail":"boom"}]}`)
			return
		case "bkt_missing":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"status":"404","title":"Not Found","detail":"Couldn't find bucket"}]}`)
			return
		}
		n := strings.TrimPrefix(id, "bkt_")
		fmt.Fprintf(w, `{"data":{"id":%q,"type":"object_storage_metrics","attributes":{"period":{"start":"2026-09-01T00:00:00Z","end":"2026-09-30T23:59:59Z"},"storage":{"consumed":%s0,"current":%s,"unit":"GB"},"estimated_cost":{"amount":%s.5,"currency":"USD"}}}}`, id, n, n, n)
	}))
}

func TestFetchMetricsFanOut(t *testing.T) {
	var peak int32
	srv := fakeMetricsAPI(t, &peak)
	defer srv.Close()

	api := sdk.New(sdk.WithServerURL(srv.URL), sdk.WithSecurity("test-token"))
	var buckets []*objectstorage.Bucket
	for i := 1; i <= 20; i++ {
		buckets = append(buckets, &objectstorage.Bucket{ID: fmt.Sprintf("bkt_%d", i), Name: fmt.Sprintf("b%d", i), StorageClass: "standard", ProjectSlug: "proj"})
	}
	buckets = append(buckets,
		&objectstorage.Bucket{ID: "bkt_err", Name: "broken"},
		&objectstorage.Bucket{ID: "bkt_missing", Name: "gone"},
	)
	sites := map[string]string{"bkt_1": "DAL", "bkt_2": "TYO4"}

	results := fetchMetrics(context.Background(), api, buckets, sites, nil)
	if len(results) != len(buckets) {
		t.Fatalf("got %d results, want %d", len(results), len(buckets))
	}
	if peak > metricsConcurrency {
		t.Errorf("peak concurrency %d exceeds %d", peak, metricsConcurrency)
	}
	if peak < 2 {
		t.Errorf("expected parallel requests, peak was %d", peak)
	}

	var failed int
	for i, res := range results {
		if res.Bucket != buckets[i] {
			t.Errorf("result %d out of order: %s", i, res.Bucket.ID)
		}
		switch res.Bucket.ID {
		case "bkt_err":
			failed++
			if res.Err == nil || exitcode.Of(res.Err) != exitcode.Generic {
				t.Errorf("bkt_err: err = %v", res.Err)
			}
		case "bkt_missing":
			failed++
			if res.Err == nil || exitcode.Of(res.Err) != exitcode.NotFound {
				t.Errorf("bkt_missing: err = %v (exit %d)", res.Err, exitcode.Of(res.Err))
			} else if !strings.Contains(res.Err.Error(), "gone (bkt_missing)") || !strings.Contains(res.Err.Error(), "Couldn't find bucket") {
				t.Errorf("bkt_missing: message should name the bucket and carry the API detail: %v", res.Err)
			}
		default:
			if res.Err != nil {
				t.Errorf("%s: unexpected error %v", res.Bucket.ID, res.Err)
				continue
			}
			n := strings.TrimPrefix(res.Bucket.ID, "bkt_")
			if res.Row.Bucket != res.Bucket.Name || res.Row.ID != res.Bucket.ID || res.Row.Project != "proj" {
				t.Errorf("row identity wrong: %+v", res.Row)
			}
			if res.Row.CurrentGB == nil || fmt.Sprint(*res.Row.CurrentGB) != n {
				t.Errorf("%s current_gb = %v", res.Bucket.ID, res.Row.CurrentGB)
			}
			if res.Row.CostAmount == nil || res.Row.Currency != "USD" {
				t.Errorf("%s cost = %v %s", res.Bucket.ID, res.Row.CostAmount, res.Row.Currency)
			}
			if res.Row.PeriodStart == nil || res.Row.PeriodEnd == nil {
				t.Errorf("%s period missing", res.Bucket.ID)
			}
		}
	}
	if failed != 2 {
		t.Errorf("failed = %d, want 2", failed)
	}
	if results[0].Row.Site != "DAL" || results[1].Row.Site != "TYO4" || results[2].Row.Site != "" {
		t.Errorf("site mapping wrong: %q %q %q", results[0].Row.Site, results[1].Row.Site, results[2].Row.Site)
	}
}

// TestDateLabelIsUTC covers the billing-period boundary fix: the API reports
// them in UTC, so rendering them in the local zone moved them a day west of
// Greenwich (UTC-3 showed the previous month's last day).
func TestDateLabelIsUTC(t *testing.T) {
	restore := time.Local
	time.Local = time.FixedZone("UTC-3", -3*60*60)
	defer func() { time.Local = restore }()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if got := dateLabel(&start); got != "2026-09-01" {
		t.Errorf("dateLabel = %q, want 2026-09-01 (the UTC date the API reported)", got)
	}
	if got := dateLabel(nil); got != "" {
		t.Errorf("dateLabel(nil) = %q, want empty", got)
	}
}
