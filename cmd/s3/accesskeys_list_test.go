package s3

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestListMergesRawScopes(t *testing.T) {
	f := newFakeAPI(t)
	f.keys["proj_1"] = keysDocument

	keys, err := f.client().list(context.Background(), "proj_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("want 3 keys, got %d: %+v", len(keys), keys)
	}
	// One GET per project: the raw document already carries every field.
	reqs := f.Requests()
	if len(reqs) != 1 || reqs[0].Method != "GET" || reqs[0].Path != "/storage/access_keys" || reqs[0].Query.Get("project") != "proj_1" {
		t.Fatalf("list must issue exactly one GET /storage/access_keys, got %+v", reqs)
	}
	if !strings.HasPrefix(reqs[0].Query.Get("project"), "proj_") {
		t.Fatalf("project query wrong: %+v", reqs[0].Query)
	}
	byID := map[string]apiKey{}
	for _, k := range keys {
		byID[k.AccessKeyID] = k
	}
	ops := byID["OPSKEYID000000000001"]
	if ops.Access != "fullaccess" || ops.StorageClass != "standard" || len(ops.Buckets) != 2 || ops.scope() != config.ScopeFullAccess {
		t.Fatalf("ops key not merged with raw scope: %+v", ops)
	}
	ci := byID["CIKEYID0000000000002"]
	if ci.Access != "rw" || ci.scope() != config.ScopeLimitedAccess || ci.Buckets[0] != "backups-7f3a" {
		t.Fatalf("ci key not merged: %+v", ci)
	}
	hp := byID["HPKEYID0000000000003"]
	if hp.StorageClass != "high_performance" || hp.Site != "TYO4" || hp.Access != "readonly" {
		t.Fatalf("hp key wrong: %+v", hp)
	}

	// Rows: SAVED comes from the profile match, SCOPE from the raw access.
	rows := []keyRow{}
	saved := map[string]string{"CIKEYID0000000000002": "ci-deploy"}
	for _, k := range keys {
		rows = append(rows, newKeyRow(k, saved, map[string]string{"proj_1": "my-project"}))
	}
	var buf bytes.Buffer
	writeKeyTable(&buf, rows, false)
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "NAME") || !strings.Contains(lines[0], "SAVED") {
		t.Fatalf("unexpected table:\n%s", out)
	}
	if !strings.Contains(out, "ci-deploy") || !strings.Contains(out, "yes") || !strings.Contains(out, "backups-7f3a") || !strings.Contains(out, "all") {
		t.Fatalf("table missing columns:\n%s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("listing must never mention secrets:\n%s", out)
	}
	for _, r := range rows {
		if r.Project != "my-project" {
			t.Fatalf("project slug not applied: %+v", r)
		}
	}
}

// TestListWithoutScopeFields replaces the former "raw scopes unavailable"
// test: listing is now a single raw request, so there is no second call to
// degrade from. Records without `access`/`buckets` (older backends) must
// still list and render '-' for the unknown scope.
func TestListWithoutScopeFields(t *testing.T) {
	f := newFakeAPI(t)
	f.keys["proj_1"] = `{"data":{"standard":[{"name":"plain","username":"p@example.com","access_key_id":"PLAINKEYID0000000001","status":"Active","created_at":"2026-09-01T10:00:00Z"}],"high_performance":[]}}`

	keys, err := f.client().list(context.Background(), "proj_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Access != "" || keys[0].StorageClass != "standard" || keys[0].Site != "" {
		t.Fatalf("listing must tolerate records without scope fields: %+v", keys)
	}
	row := newKeyRow(keys[0], nil, nil)
	if row.scopeCell() != emptyCell || row.bucketsCell() != emptyCell {
		t.Fatalf("unknown scope must render the shared placeholder: %+v", row)
	}
	if len(f.Requests()) != 1 {
		t.Fatalf("want exactly one request, got %d", len(f.Requests()))
	}
}

func TestListProjectNotFound(t *testing.T) {
	f := newFakeAPI(t)
	_, err := f.client().list(context.Background(), "nope")
	if err == nil || exitcode.Of(err) != exitcode.NotFound {
		t.Fatalf("404 must map to exit 3, got %v", err)
	}
	// Without a token the raw call fails locally with exit 4 (no request made).
	api := f.client()
	api.token = ""
	if _, err := api.list(context.Background(), "proj_1"); err == nil || exitcode.Of(err) != exitcode.Credentials {
		t.Fatalf("missing token must be exit 4, got %v", err)
	}
}

// TestSiteFilterTreatsEmptyAPISiteAsWildcard covers F17: standard keys the
// API lists without a site must not vanish behind --region, and the single
// match gets the requested site filled in.
func TestSiteFilterTreatsEmptyAPISiteAsWildcard(t *testing.T) {
	noSite := apiKey{Name: "ops", AccessKeyID: "A1", Username: "u1", StorageClass: "standard", Project: "proj_1"}
	dal := apiKey{Name: "ci", AccessKeyID: "A2", Username: "u2", StorageClass: "standard", Site: "DAL", Project: "proj_1"}
	tyo := apiKey{Name: "fast", AccessKeyID: "A3", Username: "u3", StorageClass: "high_performance", Site: "TYO4", Project: "proj_1"}
	filter := listFilter{Site: "DAL"}
	if !filter.matchAPI(noSite) || !filter.matchAPI(dal) || filter.matchAPI(tyo) {
		t.Fatalf("empty API site must match any --region; a differing site must not")
	}
	if got := filter.withSite(noSite); got.Site != "DAL" {
		t.Fatalf("withSite must fill the empty site, got %q", got.Site)
	}
	if got := filter.withSite(tyo); got.Site != "TYO4" {
		t.Fatalf("withSite must not override an API site, got %q", got.Site)
	}
	keys := []apiKey{noSite, dal, tyo}
	if got := findAPIKeys("ops", keys, filter); len(got) != 1 || got[0].AccessKeyID != "A1" {
		t.Fatalf("get with --region must still find the standard key without a site: %+v", got)
	}
	target, err := resolveDeleteTarget("ops", keys, nil, filter)
	if err != nil || target.AccessKeyID != "A1" || target.Site != "DAL" {
		t.Fatalf("delete with --region must resolve the key and carry the site, got %+v err=%v", target, err)
	}
}

func TestResolveDeleteTarget(t *testing.T) {
	keys := []apiKey{
		{Name: "backup", AccessKeyID: "A1", Username: "u1", StorageClass: "standard", Site: "DAL", Project: "proj_1"},
		{Name: "backup", AccessKeyID: "A2", Username: "u2", StorageClass: "high_performance", Site: "TYO4", Project: "proj_1"},
		{Name: "other", AccessKeyID: "A3", Username: "u3", StorageClass: "standard", Project: "proj_2"},
	}
	saved := map[string]config.StoredAccessKey{
		"mine":     {AccessKeyID: "A3", Username: "u3", StorageClass: "standard", ProjectID: "proj_2", Scope: config.ScopeFullAccess},
		"orphaned": {AccessKeyID: "ZZ", Username: "uz", StorageClass: "standard", ProjectID: "proj_9", Scope: config.ScopeFullAccess},
	}

	_, err := resolveDeleteTarget("backup", keys, saved, listFilter{})
	if err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "--storage-class") {
		t.Fatalf("ambiguous name must exit 2 with the disambiguating flags, got %v", err)
	}
	k, err := resolveDeleteTarget("backup", keys, saved, listFilter{StorageClass: "high_performance"})
	if err != nil || k.AccessKeyID != "A2" {
		t.Fatalf("--storage-class must disambiguate, got %+v err=%v", k, err)
	}
	k, err = resolveDeleteTarget("backup", keys, saved, listFilter{Site: "dal"})
	if err != nil || k.AccessKeyID != "A1" {
		t.Fatalf("--region must disambiguate, got %+v err=%v", k, err)
	}
	k, err = resolveDeleteTarget("u2", keys, saved, listFilter{})
	if err != nil || k.AccessKeyID != "A2" {
		t.Fatalf("username must resolve, got %+v err=%v", k, err)
	}
	// A saved name resolves to its access key ID on the API.
	k, err = resolveDeleteTarget("mine", keys, saved, listFilter{})
	if err != nil || k.AccessKeyID != "A3" || k.Username != "u3" {
		t.Fatalf("saved name must resolve through the API, got %+v err=%v", k, err)
	}
	// A saved key the API no longer lists is deleted with its stored metadata.
	k, err = resolveDeleteTarget("orphaned", keys, saved, listFilter{})
	if err != nil || k.AccessKeyID != "ZZ" || k.Username != "uz" || k.Project != "proj_9" {
		t.Fatalf("orphaned saved key must resolve from the profile, got %+v err=%v", k, err)
	}
	if _, err := resolveDeleteTarget("missing", keys, saved, listFilter{}); err == nil || exitcode.Of(err) != exitcode.NotFound {
		t.Fatalf("unknown token must exit 3, got %v", err)
	}
}

func TestDeleteSendsRegionOnlyForHighPerformance(t *testing.T) {
	f := newFakeAPI(t)
	api := f.client()
	ctx := context.Background()
	if err := api.delete(ctx, "u1", "standard", "proj_1", "DAL"); err != nil {
		t.Fatal(err)
	}
	if err := api.delete(ctx, "u2", "high_performance", "proj_1", "TYO4"); err != nil {
		t.Fatal(err)
	}
	if err := api.delete(ctx, "u2", "high_performance", "proj_1", ""); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("high_performance without site must fail with exit 2, got %v", err)
	}
	if err := api.delete(ctx, "", "standard", "proj_1", ""); err == nil {
		t.Fatal("missing username must fail")
	}
	reqs := f.Requests()
	if len(reqs) != 2 {
		t.Fatalf("want 2 DELETE calls, got %d", len(reqs))
	}
	if reqs[0].Path != "/storage/access_keys/u1/standard" || reqs[0].Query.Get("project") != "proj_1" || reqs[0].Query.Has("region") {
		t.Fatalf("standard delete request wrong: %+v", reqs[0])
	}
	if reqs[1].Path != "/storage/access_keys/u2/high_performance" || reqs[1].Query.Get("region") != "TYO4" {
		t.Fatalf("high_performance delete request wrong: %+v", reqs[1])
	}

	f.deleteStatus = 404
	if err := api.delete(ctx, "gone", "standard", "proj_1", ""); err == nil || exitcode.Of(err) != exitcode.NotFound {
		t.Fatalf("404 must map to exit 3, got %v", err)
	}
}

func TestSavedKeyRowNeverHoldsSecret(t *testing.T) {
	k := config.StoredAccessKey{AccessKeyID: "ID", SecretAccessKey: "topsecret", StorageClass: "standard", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": "rw"}, Source: "import", CreatedAt: mustTime(t, "2026-09-07T10:00:00Z")}
	row := newSavedKeyRow("mine", k, "test")
	var buf bytes.Buffer
	writeSavedKeyTable(&buf, []savedKeyRow{row}, true)
	if strings.Contains(buf.String(), "topsecret") {
		t.Fatalf("secret leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "bkt_1=rw") || !strings.Contains(buf.String(), "API") {
		t.Fatalf("unexpected saved table:\n%s", buf.String())
	}
	var out bytes.Buffer
	printSavedKeyDetails(&out, row)
	if strings.Contains(out.String(), "topsecret") || !strings.Contains(out.String(), "Access Key ID:   ID") {
		t.Fatalf("unexpected details:\n%s", out.String())
	}
}
