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
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/types"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

func TestParseUsageWindow(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

	start, end, err := parseUsageWindow("", "", now)
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if !start.Equal(now.AddDate(0, 0, -30)) || !end.Equal(now) {
		t.Errorf("defaults = %s..%s, want 30d..now", start, end)
	}

	start, end, err = parseUsageWindow("7d", "now", now)
	if err != nil || !start.Equal(now.Add(-7*24*time.Hour)) || !end.Equal(now) {
		t.Errorf("7d..now = %s..%s (%v)", start, end, err)
	}

	start, end, err = parseUsageWindow("2026-08-01", "2026-08-31", now)
	if err != nil || start.Format("2006-01-02") != "2026-08-01" || end.Format("2006-01-02") != "2026-08-31" {
		t.Errorf("absolute dates = %s..%s (%v)", start, end, err)
	}

	start, end, err = parseUsageWindow("2w", "1d", now)
	if err != nil || !start.Equal(now.Add(-14*24*time.Hour)) || !end.Equal(now.Add(-24*time.Hour)) {
		t.Errorf("2w..1d = %s..%s (%v)", start, end, err)
	}

	for _, c := range [][2]string{{"yesterday", ""}, {"", "soon"}, {"1d", "7d"}, {"2026-09-10", "2026-09-01"}} {
		_, _, err := parseUsageWindow(c[0], c[1], now)
		if err == nil {
			t.Errorf("parseUsageWindow(%q, %q): expected error", c[0], c[1])
			continue
		}
		if exitcode.Of(err) != exitcode.Usage {
			t.Errorf("parseUsageWindow(%q, %q): exit %d, want %d", c[0], c[1], exitcode.Of(err), exitcode.Usage)
		}
	}
}

func TestValidateGroupBy(t *testing.T) {
	for in, want := range map[string]string{"": "day", "day": "day", "Bucket": "bucket", " tier ": "tier", "REGION": "region"} {
		got, err := validateGroupBy(in)
		if err != nil || got != want {
			t.Errorf("validateGroupBy(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := validateGroupBy("week"); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Errorf("validateGroupBy(week) = %v", err)
	}
}

func usageDatum(date, storageID, tier, region string, bytes int64) components.StorageUsageData {
	d := types.MustDateFromString(date)
	return components.StorageUsageData{Attributes: &components.StorageUsageAttributes{
		Date: &d, StorageID: ptrS(storageID), Tier: ptrS(tier), Region: ptrS(region), Bytes: ptrI(bytes), StorageType: ptrS("object"),
	}}
}

// usageFixtureData is the raw API answer for one project. The storage_id the
// API returns is a numeric string ("2028"), not the bkt_ id, which is why the
// names map is keyed by those numbers (see usageStorageIDNames).
func usageFixtureData() []components.StorageUsageData {
	return []components.StorageUsageData{
		usageDatum("2026-09-01", "2028", "standard", "DAL", 1000),
		usageDatum("2026-09-01", "42", "high", "TYO4", 100),
		usageDatum("2026-09-02", "2028", "standard", "DAL", 2000),
		usageDatum("2026-09-02", "42", "high", "TYO4", 200),
		usageDatum("2026-09-03", "2028", "standard", "DAL", 3000),
		usageDatum("2026-09-03", "7", "standard", "DAL", 50), // unknown storage id
		{}, // row without attributes is skipped
	}
}

func usageFixture() []usageSample {
	names := map[string]string{"2028": "backups", "42": "logs"}
	return usageSamples(usageFixtureData(), "my-project", names, "")
}

func TestUsageSamples(t *testing.T) {
	samples := usageFixture()
	if len(samples) != 6 {
		t.Fatalf("got %d samples, want 6", len(samples))
	}
	if samples[0].Bucket != "backups" || samples[1].Bucket != "logs" || samples[5].Bucket != "7" {
		t.Errorf("bucket names = %q %q %q", samples[0].Bucket, samples[1].Bucket, samples[5].Bucket)
	}
	if !samples[0].Named || !samples[1].Named || samples[5].Named {
		t.Errorf("named flags = %v %v %v, want true true false", samples[0].Named, samples[1].Named, samples[5].Named)
	}
	if samples[0].StorageID != "2028" {
		t.Errorf("storage id = %q, want the API's numeric id", samples[0].StorageID)
	}
	if samples[0].Date != "2026-09-01" || samples[0].Project != "my-project" || samples[0].Bytes != 1000 {
		t.Errorf("sample = %+v", samples[0])
	}
	// The bkt_ id never appears in usage rows, so a map keyed by it resolves nothing.
	if byBkt := usageSamples(usageFixtureData(), "p", map[string]string{"bkt_1": "backups"}, ""); byBkt[0].Named {
		t.Errorf("bkt_ keyed names must not match numeric storage ids: %+v", byBkt[0])
	}
	// With a bucket filter, unknown ids fall back to the filtered bucket's name.
	filtered := usageSamples([]components.StorageUsageData{usageDatum("2026-09-01", "99", "standard", "DAL", 1)}, "p", nil, "backups")
	if filtered[0].Bucket != "backups" || !filtered[0].Named {
		t.Errorf("fallback bucket = %+v", filtered[0])
	}
}

// usageStorageIDs is the fake API's bkt_ id -> numeric storage_id table.
var usageStorageIDs = map[string]string{"bkt_1": "2028", "bkt_2": "42", "bkt_3": "300"}

// fakeUsageAPI serves GET /storage/usage. With filter[storage_id]=<bkt_ id> it
// answers rows carrying that bucket's numeric storage_id (the real API
// behaviour); bkt_err fails with 500 and bkt_empty has no rows. It records
// the peak concurrency and the projects it was asked for.
func fakeUsageAPI(t *testing.T, peak *int32, projects *sync.Map) *httptest.Server {
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

		if r.URL.Path != "/storage/usage" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		if q.Get("filter[start_date]") == "" || q.Get("filter[end_date]") == "" {
			t.Errorf("lookup must carry the usage window, got %s", r.URL.RawQuery)
		}
		id := q.Get("filter[storage_id]")
		projects.Store(id, q.Get("filter[project]"))
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch id {
		case "bkt_err":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"status":"500","title":"Internal Server Error","detail":"boom"}]}`)
			return
		case "bkt_empty":
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		sid := usageStorageIDs[id]
		fmt.Fprintf(w, `{"data":[
			{"id":"u1","type":"storage_usage","attributes":{"date":"2026-09-01","storage_id":%q,"project_id":"1","storage_type":"object","tier":"standard","region":"DAL","bytes":1000}},
			{"id":"u2","type":"storage_usage","attributes":{"date":"2026-09-02","storage_id":%q,"project_id":"1","storage_type":"object","tier":"standard","region":"DAL","bytes":2000}}
		]}`, sid, sid)
	}))
}

func TestUsageStorageIDNames(t *testing.T) {
	var peak int32
	var projects sync.Map
	srv := fakeUsageAPI(t, &peak, &projects)
	defer srv.Close()
	api := sdk.New(sdk.WithServerURL(srv.URL), sdk.WithSecurity("test-token"))

	buckets := []*objectstorage.Bucket{
		{ID: "bkt_1", Name: "backups", ProjectSlug: "proj"},
		{ID: "bkt_2", Name: "logs", ProjectSlug: "proj"},
		{ID: "bkt_3", Name: "no-project"}, // falls back to the command's project
		{ID: "bkt_err", Name: "broken", ProjectSlug: "proj"},
		{ID: "bkt_empty", Name: "idle", ProjectSlug: "proj"},
		{ID: "", Name: "no-id"}, // skipped: nothing to filter on
	}
	// Pad with more buckets than the concurrency bound to observe the fan-out.
	for i := 10; i < 30; i++ {
		buckets = append(buckets, &objectstorage.Bucket{ID: fmt.Sprintf("bkt_%d", i), Name: fmt.Sprintf("b%d", i), ProjectSlug: "proj"})
	}
	start, end := types.NewDate(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)), types.NewDate(time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	names := usageStorageIDNames(context.Background(), api, buckets, "fallback-proj", start, end, nil)

	want := map[string]string{"2028": "backups", "42": "logs", "300": "no-project"}
	for sid, name := range want {
		if names[sid] != name {
			t.Errorf("names[%q] = %q, want %q", sid, names[sid], name)
		}
	}
	// Padding buckets have no storage id in the fake table and all answer "",
	// which must not be recorded; failures and empty answers stay unmapped.
	if _, ok := names[""]; ok {
		t.Errorf("empty storage ids must not be recorded: %v", names)
	}
	for sid, name := range names {
		if _, ok := want[sid]; !ok {
			t.Errorf("unexpected mapping %q -> %q", sid, name)
		}
	}
	if peak > usageLookupConcurrency {
		t.Errorf("peak concurrency %d exceeds %d", peak, usageLookupConcurrency)
	}
	if peak < 2 {
		t.Errorf("expected parallel requests, peak was %d", peak)
	}
	if p, _ := projects.Load("bkt_1"); p != "proj" {
		t.Errorf("bkt_1 queried with project %v, want proj", p)
	}
	if p, _ := projects.Load("bkt_3"); p != "fallback-proj" {
		t.Errorf("bucket without project should use the command's project, got %v", p)
	}
	if _, asked := projects.Load(""); asked {
		t.Errorf("a bucket without id must not trigger a lookup")
	}

	// The mapped names resolve the rows of the unfiltered call.
	samples := usageSamples(usageFixtureData(), "proj", names, "")
	if samples[0].Bucket != "backups" || samples[1].Bucket != "logs" || samples[5].Bucket != "7" {
		t.Errorf("resolved buckets = %q %q %q", samples[0].Bucket, samples[1].Bucket, samples[5].Bucket)
	}
}

func TestAggregateUsageRawStorageIDs(t *testing.T) {
	// No names at all (too many buckets for the lookup): the table shows the
	// API's storage_id under "Storage ID" instead of an id-filled Bucket column.
	raw := aggregateUsage(usageSamples(usageFixtureData(), "p", nil, ""), groupByBucket, false)
	if len(raw) != 3 {
		t.Fatalf("got %d rows, want 3", len(raw))
	}
	row := raw[0].TableRow()
	if _, ok := row["bucket"]; ok {
		t.Errorf("unresolved rows must not have a Bucket column: %+v", row)
	}
	if got := row["storage_id"]; got.Label != "Storage ID" || got.Value != "2028" {
		t.Errorf("storage id column = %+v", got)
	}
	// JSON keeps both fields; bucket falls back to the storage id.
	out, _ := json.Marshal(raw[0])
	if !strings.Contains(string(out), `"storage_id":"2028"`) || !strings.Contains(string(out), `"bucket":"2028"`) {
		t.Errorf("json = %s", out)
	}

	// As soon as one name resolves the Bucket column is back (unknown ids are
	// shown as-is inside it).
	named := aggregateUsage(usageFixture(), groupByBucket, false)
	for _, r := range named {
		row := r.TableRow()
		if _, ok := row["storage_id"]; ok {
			t.Errorf("resolved rows must not have a Storage ID column: %+v", row)
		}
		if row["bucket"].Label != "Bucket" {
			t.Errorf("bucket column = %+v", row["bucket"])
		}
	}
	if named[2].TableRow()["bucket"].Value != "7" {
		t.Errorf("unknown id inside the Bucket column = %q", named[2].TableRow()["bucket"].Value)
	}
}

func TestAggregateUsageByDay(t *testing.T) {
	rows := aggregateUsage(usageFixture(), groupByDay, false)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}
	wantBytes := map[string]int64{"2026-09-01": 1100, "2026-09-02": 2200, "2026-09-03": 3050}
	for i, r := range rows {
		if r.Bytes != wantBytes[r.Date] {
			t.Errorf("%s bytes = %d, want %d", r.Date, r.Bytes, wantBytes[r.Date])
		}
		if r.Project != "my-project" {
			t.Errorf("project = %q", r.Project)
		}
		// Several buckets per day: bucket is never uniform; tier/region are
		// blank when mixed (09-01, 09-02) and kept when shared (09-03).
		if r.Bucket != "" || r.StorageID != "" {
			t.Errorf("%s: mixed buckets should be blank: %+v", r.Date, r)
		}
		if r.Date == "2026-09-03" {
			if r.Tier != "standard" || r.Region != "DAL" {
				t.Errorf("%s: uniform tier/region should be kept: %+v", r.Date, r)
			}
		} else if r.Tier != "" || r.Region != "" {
			t.Errorf("%s: mixed tier/region should be blank: %+v", r.Date, r)
		}
		if i > 0 && rows[i-1].Date > r.Date {
			t.Errorf("rows not sorted by date: %v", rows)
		}
		if r.Days != 0 || r.AvgBytes != 0 {
			t.Errorf("day rows must not carry days/avg: %+v", r)
		}
		row := r.TableRow()
		if _, ok := row["date"]; !ok {
			t.Errorf("day grouping should have a date column")
		}
		if _, ok := row["days"]; ok {
			t.Errorf("day grouping should not have a days column")
		}
	}
}

func TestAggregateUsageByBucket(t *testing.T) {
	rows := aggregateUsage(usageFixture(), groupByBucket, true)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}
	// Sorted by bytes desc.
	if rows[0].Bucket != "backups" || rows[0].Bytes != 6000 || rows[0].Days != 3 || rows[0].AvgBytes != 2000 {
		t.Errorf("backups row = %+v", rows[0])
	}
	if rows[0].Tier != "standard" || rows[0].Region != "DAL" || rows[0].StorageID != "2028" || rows[0].Date != "" {
		t.Errorf("uniform dimensions should be kept: %+v", rows[0])
	}
	if rows[1].Bucket != "logs" || rows[1].Bytes != 300 || rows[1].Days != 2 || rows[1].AvgBytes != 150 {
		t.Errorf("logs row = %+v", rows[1])
	}
	if rows[2].Bucket != "7" || rows[2].Bytes != 50 || rows[2].Days != 1 {
		t.Errorf("unknown bucket row = %+v", rows[2])
	}
	// -H fills the human sizes and the table shows them.
	if rows[0].Size != "5.9 KiB" || rows[0].AvgSize != "2.0 KiB" {
		t.Errorf("human sizes = %q %q", rows[0].Size, rows[0].AvgSize)
	}
	row := rows[0].TableRow()
	if row["bytes"].Value != "5.9 KiB" || row["avg_bytes"].Value != "2.0 KiB" || row["days"].Value != "3" {
		t.Errorf("table row = %+v", row)
	}
	if _, ok := row["date"]; ok {
		t.Errorf("bucket grouping should not have a date column")
	}
}

func TestAggregateUsageByTierAndRegion(t *testing.T) {
	tiers := aggregateUsage(usageFixture(), groupByTier, false)
	if len(tiers) != 2 || tiers[0].Tier != "standard" || tiers[0].Bytes != 6050 || tiers[1].Tier != "high" || tiers[1].Bytes != 300 {
		t.Errorf("tier rows = %+v", tiers)
	}
	// standard spans two buckets: bucket is blank, region is uniform.
	if tiers[0].Bucket != "" || tiers[0].Region != "DAL" {
		t.Errorf("standard tier dimensions = %+v", tiers[0])
	}
	regions := aggregateUsage(usageFixture(), groupByRegion, false)
	if len(regions) != 2 || regions[0].Region != "DAL" || regions[0].Bytes != 6050 || regions[1].Region != "TYO4" || regions[1].Bytes != 300 {
		t.Errorf("region rows = %+v", regions)
	}

	// Projects stay separate under --all-projects.
	mixed := append(usageFixture(), usageSample{Date: "2026-09-01", Project: "other", StorageID: "x", Bucket: "x", Tier: "standard", Region: "DAL", Bytes: 1})
	byTier := aggregateUsage(mixed, groupByTier, false)
	if len(byTier) != 3 {
		t.Errorf("expected a separate row per project, got %+v", byTier)
	}
	if empty := aggregateUsage(nil, groupByDay, false); len(empty) != 0 {
		t.Errorf("no samples should yield no rows, got %+v", empty)
	}
}

func TestUsageRowJSONShape(t *testing.T) {
	rows := aggregateUsage(usageFixture(), groupByBucket, false)
	raw, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"project":"my-project"`, `"bucket":"backups"`, `"storage_id":"2028"`, `"tier":"standard"`, `"region":"DAL"`, `"bytes":6000`, `"days":3`, `"avg_bytes":2000`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("json %s lacks %s", raw, want)
		}
	}
	if strings.Contains(string(raw), `"size"`) || strings.Contains(string(raw), `"date"`) || strings.Contains(string(raw), "groupBy") {
		t.Errorf("unexpected fields in %s", raw)
	}
}

// TestUsageSamplesSkipsOtherStorageTypes covers the aggregation fix: the
// endpoint has no storage_type filter and returns object, file and block rows,
// so summing everything would inflate object storage usage.
func TestUsageSamplesSkipsOtherStorageTypes(t *testing.T) {
	str := func(s string) *string { return &s }
	i64 := func(n int64) *int64 { return &n }
	data := []components.StorageUsageData{
		{Attributes: &components.StorageUsageAttributes{StorageID: str("1"), StorageType: str("object"), Bytes: i64(100)}},
		{Attributes: &components.StorageUsageAttributes{StorageID: str("2"), StorageType: str("file"), Bytes: i64(900)}},
		{Attributes: &components.StorageUsageAttributes{StorageID: str("3"), StorageType: str("block"), Bytes: i64(500)}},
		// An older payload without the field is kept: this endpoint is the
		// object storage one, so an unset type is treated as object.
		{Attributes: &components.StorageUsageAttributes{StorageID: str("4"), Bytes: i64(7)}},
	}
	got := usageSamples(data, "proj_1", nil, "")
	if len(got) != 2 {
		t.Fatalf("kept %d samples, want the 2 object rows: %+v", len(got), got)
	}
	var total int64
	for _, s := range got {
		total += s.Bytes
	}
	if total != 107 {
		t.Errorf("total = %d bytes, want 107 (file/block rows must not be summed)", total)
	}
}
