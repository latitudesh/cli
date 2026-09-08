package s3

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

func TestParseBucketSpec(t *testing.T) {
	cases := []struct {
		in    string
		token string
		perm  string
		fail  bool
	}{
		{"backups", "backups", config.PermissionRW, false},
		{"backups=rw", "backups", config.PermissionRW, false},
		{"logs=readonly", "logs", config.PermissionReadOnly, false},
		{"logs:readonly", "logs", config.PermissionReadOnly, false},
		{"logs:ro", "logs", config.PermissionReadOnly, false},
		{"s3://backups/", "backups", config.PermissionRW, false},
		{"bkt_1Gjbang9n0L2w=readonly", "bkt_1Gjbang9n0L2w", config.PermissionReadOnly, false},
		{"backups=write", "backups", config.PermissionRW, false},
		{"backups=admin", "", "", true},
		{"=rw", "", "", true},
		{"", "", "", true},
		// A key part is a usage error: access keys cover whole buckets.
		{"backups/2026/", "", "", true},
		{"s3://backups/dump.sql=rw", "", "", true},
		{"backups/dir:readonly", "", "", true},
	}
	for _, c := range cases {
		spec, err := parseBucketSpec(c.in)
		if c.fail {
			if err == nil || exitcode.Of(err) != exitcode.Usage {
				t.Errorf("%q: want usage error, got spec=%+v err=%v", c.in, spec, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if spec.Token != c.token || spec.Permission != c.perm {
			t.Errorf("%q: got %+v, want token=%q perm=%q", c.in, spec, c.token, c.perm)
		}
	}
}

func TestParseBucketSpecsRejectsDuplicatesAndSplitsCommas(t *testing.T) {
	specs, err := parseBucketSpecs([]string{"a=rw,b=readonly", "c"})
	if err != nil || len(specs) != 3 {
		t.Fatalf("got %+v err=%v", specs, err)
	}
	if specs[1].Token != "b" || specs[1].Permission != config.PermissionReadOnly {
		t.Fatalf("comma split failed: %+v", specs)
	}
	if _, err := parseBucketSpecs([]string{"a", "a=readonly"}); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("duplicate must fail with exit 2, got %v", err)
	}
}

func TestValidateSameGroup(t *testing.T) {
	std := func(name, project string) scopedBucket {
		return scopedBucket{Bucket: &objectstorage.Bucket{Name: name, StorageClass: objectstorage.ClassStandard, ProjectID: project, Site: "DAL"}, Permission: "rw"}
	}
	hp := func(name, project, site string) scopedBucket {
		return scopedBucket{Bucket: &objectstorage.Bucket{Name: name, StorageClass: objectstorage.ClassHighPerformance, ProjectID: project, Site: site}, Permission: "rw"}
	}
	if err := validateSameGroup([]scopedBucket{std("a", "p1"), std("b", "p1")}); err != nil {
		t.Fatalf("same class/project must pass: %v", err)
	}
	// Standard keys span sites: differing sites are fine.
	other := std("c", "p1")
	other.Bucket.Site = "NYC"
	if err := validateSameGroup([]scopedBucket{std("a", "p1"), other}); err != nil {
		t.Fatalf("standard buckets in different sites must pass: %v", err)
	}
	err := validateSameGroup([]scopedBucket{std("a", "p1"), hp("b", "p1", "TYO4")})
	if err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("mixed classes must fail with exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "class=standard") || !strings.Contains(err.Error(), "class=high_performance") || !strings.Contains(err.Error(), "a") {
		t.Fatalf("error must list the groups: %v", err)
	}
	if err := validateSameGroup([]scopedBucket{hp("a", "p1", "TYO4"), hp("b", "p1", "DAL")}); err == nil {
		t.Fatal("high_performance buckets in different sites must fail")
	}
	if err := validateSameGroup([]scopedBucket{std("a", "p1"), std("b", "p2")}); err == nil {
		t.Fatal("different projects must fail")
	}
}

func TestNormalizeCreatedBothShapes(t *testing.T) {
	req := accessKeyRequest{Project: "proj_1", StorageClass: objectstorage.ClassStandard, Site: "DAL", Name: "ci", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": "rw"}}
	s := func(v string) *string { return &v }
	wasabi := &operations.PostStorageAccessKeysResponse{Object: &operations.PostStorageAccessKeysResponseBody{Data: &operations.PostStorageAccessKeysObjectStorageData{
		Attributes: &operations.PostStorageAccessKeysObjectStorageAttributes{AccessKey: &operations.AccessKey{
			AccessKeyID: s("WASABIID"), SecretAccessKey: s("wasabi-secret"), Name: s("ci"), Status: s("Active"), Username: s("u+ci@x"),
		}},
	}}}
	k, err := normalizeCreated(wasabi, req)
	if err != nil {
		t.Fatal(err)
	}
	if k.AccessKeyID != "WASABIID" || k.SecretAccessKey != "wasabi-secret" || k.Username != "u+ci@x" || k.Scope != config.ScopeLimitedAccess || k.Buckets["bkt_1"] != "rw" {
		t.Fatalf("wasabi shape not normalized: %+v", *k)
	}

	req.StorageClass = objectstorage.ClassHighPerformance
	req.Site = "TYO4"
	vast := &operations.PostStorageAccessKeysResponse{Object: &operations.PostStorageAccessKeysResponseBody{Data: &operations.PostStorageAccessKeysObjectStorageData{
		Attributes: &operations.PostStorageAccessKeysObjectStorageAttributes{AccessKey: &operations.AccessKey{
			AccessKey: s("VASTID"), SecretKey: s("vast-secret"), Status: s("Active"),
		}},
	}}}
	k, err = normalizeCreated(vast, req)
	if err != nil {
		t.Fatal(err)
	}
	if k.AccessKeyID != "VASTID" || k.SecretAccessKey != "vast-secret" || k.Name != "ci" || k.Site != "TYO4" {
		t.Fatalf("vast shape not normalized: %+v", *k)
	}
	// Both shapes marshal to identical field names.
	b, _ := json.Marshal(k)
	for _, field := range []string{`"access_key_id"`, `"secret_access_key"`, `"storage_class"`, `"scope"`} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("JSON missing %s: %s", field, b)
		}
	}
	if strings.Contains(string(b), `"secret_key"`) || strings.Contains(string(b), `"access_key":`) {
		t.Fatalf("backend field names leaked into JSON: %s", b)
	}

	if _, err := normalizeCreated(&operations.PostStorageAccessKeysResponse{}, req); err == nil {
		t.Fatal("empty response must fail")
	}
	noSecret := &operations.PostStorageAccessKeysResponse{Object: &operations.PostStorageAccessKeysResponseBody{Data: &operations.PostStorageAccessKeysObjectStorageData{
		Attributes: &operations.PostStorageAccessKeysObjectStorageAttributes{AccessKey: &operations.AccessKey{AccessKeyID: s("X")}},
	}}}
	if _, err := normalizeCreated(noSecret, req); err == nil {
		t.Fatal("response without secret must fail")
	}
}

func TestBuildCreateRequestSortsBuckets(t *testing.T) {
	req := accessKeyRequest{Project: "p", StorageClass: objectstorage.ClassStandard, Site: "DAL", Name: "n", Scope: config.ScopeLimitedAccess,
		Buckets: map[string]string{"bkt_b": "readonly", "bkt_a": "rw"}}
	body := buildCreateRequest(req)
	a := body.Data.Attributes
	if a.AccessScope != operations.AccessScopeLimitedAccess || a.Region != "DAL" || len(a.BucketPermissions) != 2 {
		t.Fatalf("unexpected attributes: %+v", a)
	}
	if a.BucketPermissions[0].BucketID != "bkt_a" || a.BucketPermissions[0].Permission != operations.PermissionRw || a.BucketPermissions[1].Permission != operations.PermissionReadonly {
		t.Fatalf("permissions not sorted/mapped: %+v", a.BucketPermissions)
	}
	req.Scope = config.ScopeFullAccess
	if body := buildCreateRequest(req); len(body.Data.Attributes.BucketPermissions) != 0 {
		t.Fatal("fullaccess must not send bucket_permissions")
	}
}

func TestAccessKeyRequestValidate(t *testing.T) {
	base := accessKeyRequest{Project: "p", StorageClass: objectstorage.ClassStandard, Site: "DAL", Name: "n", Scope: config.ScopeFullAccess}
	if err := base.validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	hp := base
	hp.StorageClass = objectstorage.ClassHighPerformance
	hp.Site = ""
	err := hp.validate()
	if err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "TYO4") {
		t.Fatalf("high_performance without site must name the expected value, got %v", err)
	}
	limited := base
	limited.Scope = config.ScopeLimitedAccess
	if err := limited.validate(); err == nil {
		t.Fatal("limited_access without buckets must fail")
	}
	limited.Buckets = map[string]string{"bkt_1": "admin"}
	if err := limited.validate(); err == nil {
		t.Fatal("invalid permission must fail")
	}
}

func TestDecideSecretDisplay(t *testing.T) {
	cases := []struct {
		human, save, show, explicit bool
		wantShow                    bool
		wantWarn                    bool
	}{
		{human: true, wantShow: true},                                               // TTY, not saving: shown once in clear
		{human: true, save: true, wantShow: false, wantWarn: true},                  // saved: not printed
		{human: true, save: true, show: true, wantShow: true},                       // saved + --show-secret
		{human: false, explicit: true, wantShow: true},                              // -o json explicit
		{human: false, explicit: false, wantShow: false, wantWarn: true},            // LSH_OUTPUT=json: omitted + warning
		{human: false, explicit: false, show: true, wantShow: true},                 // --show-secret overrides
		{human: false, save: true, explicit: true, wantShow: false, wantWarn: true}, // saving wins over explicit -o
		{human: false, save: true, show: true, wantShow: true},
	}
	for i, c := range cases {
		d := decideSecretDisplay(c.human, c.save, c.show, c.explicit)
		if d.Show != c.wantShow || (d.Warning != "") != c.wantWarn {
			t.Errorf("case %d (%+v): got show=%v warn=%q", i, c, d.Show, d.Warning)
		}
	}
}

func TestTruncateAndFormatHelpers(t *testing.T) {
	if got := truncateText("abcdef", 4); got != "abc…" {
		t.Fatalf("truncateText = %q", got)
	}
	if got := truncateText("abc", 4); got != "abc" {
		t.Fatalf("truncateText short = %q", got)
	}
	if got := formatPerms(map[string]string{"bkt_b": "rw", "bkt_a": "readonly"}); got != "bkt_a=readonly bkt_b=rw" {
		t.Fatalf("formatPerms = %q", got)
	}
	if !looksLikeAWSRegion("us-east-1") || looksLikeAWSRegion("TYO4") || looksLikeAWSRegion("DAL") {
		t.Fatal("AWS region detection is off")
	}
}

func TestRotatedName(t *testing.T) {
	now := mustTime(t, "2026-09-07T12:00:00Z")
	if got := rotatedName("ci-deploy", now, 1); got != "ci-deploy-20260907" {
		t.Fatalf("rotatedName = %q", got)
	}
	req, err := rotateRequest(config.StoredAccessKey{
		AccessKeyID: "OLD", StorageClass: objectstorage.ClassHighPerformance, Site: "tyo4", ProjectID: "proj_1",
		Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": "rw"},
	}, "", "", "ci-deploy-20260907")
	if err != nil {
		t.Fatal(err)
	}
	if req.Project != "proj_1" || req.Site != "TYO4" || req.Scope != config.ScopeLimitedAccess || req.Buckets["bkt_1"] != "rw" || req.Name != "ci-deploy-20260907" {
		t.Fatalf("rotateRequest = %+v", req)
	}
	if _, err := rotateRequest(config.StoredAccessKey{Scope: config.ScopeUnknown}, "p", "", "n"); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("unknown scope must fail with exit 2, got %v", err)
	}
	if _, err := rotateRequest(config.StoredAccessKey{Scope: config.ScopeFullAccess, StorageClass: "standard"}, "", "", "n"); err == nil {
		t.Fatal("missing project must fail")
	}
}

func TestValidateAccessKeyName(t *testing.T) {
	if err := validateAccessKeyName("ci-deploy"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}
	if err := validateAccessKeyName("this-name-is-way-too-long-for-the-api"); err == nil {
		t.Error("over-long name must be rejected")
	}
}

// lagError reproduces what the API returns while the backend converges:
// 404 STORAGE_RESOURCE_NOT_FOUND, humanized by objectstorage.HumanizeAPI.
func lagError() error {
	return exitcode.Errorf(exitcode.NotFound, "not found: Storage resource not found.")
}

func TestIsProvisioningLag(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{lagError(), true},
		{exitcode.Errorf(exitcode.Generic, "the API returned 502: Storage service is temporarily unavailable. Please try again."), true},
		{exitcode.Errorf(exitcode.Usage, "the API rejected the request: name size cannot be greater than 25"), false},
		{exitcode.Errorf(exitcode.NotFound, "project %q not found", "nope"), false},
	}
	for _, c := range cases {
		if got := isProvisioningLag(c.err); got != c.want {
			t.Errorf("isProvisioningLag(%q) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestRetryCreateProvisioningLag covers the fix for a key created right after
// its bucket: the first attempts fail with the propagation 404 and the same
// request is re-sent (same name) until it succeeds.
func TestRetryCreateProvisioningLag(t *testing.T) {
	restore := provisioningRetryDelays
	provisioningRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { provisioningRetryDelays = restore }()

	var names []string
	calls := 0
	req := accessKeyRequest{Name: "key-brave-otter-std", StorageClass: objectstorage.ClassStandard}
	created, err := retryCreate(context.Background(), req, true, func(r accessKeyRequest) (*createdAccessKey, error) {
		calls++
		names = append(names, r.Name)
		if calls < 3 {
			return nil, lagError()
		}
		return &createdAccessKey{Name: r.Name}, nil
	})
	if err != nil {
		t.Fatalf("retryCreate: %v", err)
	}
	if created.Name != "key-brave-otter-std" {
		t.Errorf("created name = %q, want the requested one", created.Name)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
	for _, n := range names {
		if n != "key-brave-otter-std" {
			t.Errorf("name changed to %q; a propagation retry must re-send the same request", n)
		}
	}
}

func TestRetryCreateProvisioningLagExhausted(t *testing.T) {
	restore := provisioningRetryDelays
	provisioningRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { provisioningRetryDelays = restore }()

	calls := 0
	_, err := retryCreate(context.Background(), accessKeyRequest{Name: "key-x"}, true, func(accessKeyRequest) (*createdAccessKey, error) {
		calls++
		return nil, lagError()
	})
	if err == nil {
		t.Fatal("expected an error once the retries are exhausted")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (initial + 2 retries)", calls)
	}
	if code := exitcode.Of(err); code != exitcode.NotFound {
		t.Errorf("exit code = %d, want %d", code, exitcode.NotFound)
	}
	if !strings.Contains(err.Error(), "retry in a few seconds") {
		t.Errorf("error %q should tell the user the failure is transient", err)
	}
}

func TestRetryCreateNameConflict(t *testing.T) {
	calls := 0
	var names []string
	req := accessKeyRequest{Name: "key-taken", StorageClass: objectstorage.ClassStandard}
	if _, err := retryCreate(context.Background(), req, true, func(r accessKeyRequest) (*createdAccessKey, error) {
		calls++
		names = append(names, r.Name)
		if calls < 2 {
			return nil, exitcode.Errorf(exitcode.Usage, "the API rejected the request: name has already been taken")
		}
		return &createdAccessKey{Name: r.Name}, nil
	}); err != nil {
		t.Fatalf("retryCreate: %v", err)
	}
	if len(names) != 2 || names[0] == names[1] {
		t.Errorf("names = %v; a conflict must re-roll the generated name", names)
	}

	// An explicit --name is never rewritten: the conflict surfaces instead.
	calls = 0
	if _, err := retryCreate(context.Background(), req, false, func(accessKeyRequest) (*createdAccessKey, error) {
		calls++
		return nil, exitcode.Errorf(exitcode.Usage, "the API rejected the request: name has already been taken")
	}); err == nil {
		t.Fatal("expected the conflict to surface for an explicit name")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestSleepCtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepCtx(ctx, time.Hour)
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if code := exitcode.Of(err); code != exitcode.Interrupted {
		t.Errorf("exit code = %d, want %d", code, exitcode.Interrupted)
	}
}

func TestImportHint(t *testing.T) {
	created := &createdAccessKey{AccessKeyID: "AKIAEXAMPLE"}
	limited := accessKeyRequest{Project: "my-project", StorageClass: objectstorage.ClassStandard, Scope: config.ScopeLimitedAccess}
	got := importHint("k1", created, limited, map[string]string{"logs": config.PermissionReadOnly, "backups": config.PermissionRW})
	want := "lsh s3 access-keys import --name k1 --access-key-id AKIAEXAMPLE --project my-project --storage-class standard --bucket backups=rw --bucket logs=readonly"
	if got != want {
		t.Errorf("limited hint =\n%s\nwant\n%s", got, want)
	}

	full := accessKeyRequest{Project: "p", StorageClass: objectstorage.ClassHighPerformance, Site: "TYO4", Scope: config.ScopeFullAccess}
	got = importHint("k2", created, full, nil)
	want = "lsh s3 access-keys import --name k2 --access-key-id AKIAEXAMPLE --project p --storage-class high_performance --region TYO4 --all-buckets"
	if got != want {
		t.Errorf("fullaccess hint =\n%s\nwant\n%s", got, want)
	}
}

// TestRotatedNameSequenceAndLimit covers same-day re-rotation (a deterministic
// name would collide on the API) and the 25-character name cap.
func TestRotatedNameSequenceAndLimit(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		base string
		seq  int
		want string
	}{
		{"ci-deploy", 1, "ci-deploy-20260907"},
		{"ci-deploy", 2, "ci-deploy-20260907-2"},
		{"", 1, "lsh-key-20260907"},
		// 20 chars + "-20260907" would be 29: the base is trimmed instead.
		{"backups-nightly-full", 1, "backups-nightly-20260907"},
		{"backups-nightly-full", 3, "backups-nightl-20260907-3"},
	}
	for _, c := range cases {
		got := rotatedName(c.base, now, c.seq)
		if got != c.want {
			t.Errorf("rotatedName(%q, seq=%d) = %q, want %q", c.base, c.seq, got, c.want)
		}
		if len(got) > maxAccessKeyNameLen {
			t.Errorf("rotatedName(%q, seq=%d) = %q is %d chars, over the API limit", c.base, c.seq, got, len(got))
		}
	}
}

// TestMatchSavedFiltersLikeMatchAPI locks the two rules matchSaved was missing:
// an empty site is a wildcard (as in matchAPI) and a project reference only
// filters when it is comparable with what the key stored.
func TestMatchSavedFiltersLikeMatchAPI(t *testing.T) {
	standard := config.StoredAccessKey{StorageClass: objectstorage.ClassStandard, ProjectID: "proj_1"}
	cases := []struct {
		name   string
		filter listFilter
		key    config.StoredAccessKey
		want   bool
	}{
		{"no filter", listFilter{}, standard, true},
		{"site filter, key without a site", listFilter{Site: "DAL"}, standard, true},
		{"site filter, key in another site", listFilter{Site: "DAL"}, config.StoredAccessKey{Site: "NYC"}, false},
		{"site filter, same site", listFilter{Site: "DAL"}, config.StoredAccessKey{Site: "dal"}, true},
		{"project slug vs stored ID", listFilter{Project: "my-project"}, standard, true},
		{"project ID mismatch", listFilter{Project: "proj_2"}, standard, false},
		{"project ID match", listFilter{Project: "proj_1"}, standard, true},
		{"class mismatch", listFilter{StorageClass: objectstorage.ClassHighPerformance}, standard, false},
	}
	for _, c := range cases {
		if got := c.filter.matchSaved(c.key); got != c.want {
			t.Errorf("%s: matchSaved = %v, want %v", c.name, got, c.want)
		}
	}
}
