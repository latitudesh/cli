package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

func baseMbOptions() mbOptions {
	return mbOptions{Name: "backups", Project: "my-project", Region: "DAL"}
}

func TestBuildCreateBucketRequest_Defaults(t *testing.T) {
	req, err := buildCreateBucketRequest(baseMbOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := req.Data.Attributes
	if req.Data.Type != operations.PostStorageBucketsTypeObjects {
		t.Errorf("type = %q, want objects", req.Data.Type)
	}
	if a.Name != "backups" || a.Project != "my-project" || a.Region != "DAL" {
		t.Errorf("attributes = %+v", a)
	}
	if a.StorageClass == nil || *a.StorageClass != operations.StorageClassStandard {
		t.Errorf("storage class = %v, want standard", a.StorageClass)
	}
	if a.Versioning == nil || *a.Versioning {
		t.Errorf("versioning should default to false")
	}
	if a.Locking == nil || *a.Locking {
		t.Errorf("locking should default to false")
	}
	if a.RetentionMode == nil || *a.RetentionMode != operations.RetentionModeNone {
		t.Errorf("retention mode = %v, want NONE", a.RetentionMode)
	}
	if a.RetentionPeriod != nil {
		t.Errorf("retention period should be nil, got %d", *a.RetentionPeriod)
	}
}

func TestBuildCreateBucketRequest_RegionUpperCased(t *testing.T) {
	o := baseMbOptions()
	o.Region = "tyo4"
	req, err := buildCreateBucketRequest(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Data.Attributes.Region != "TYO4" {
		t.Errorf("region = %q, want TYO4", req.Data.Attributes.Region)
	}
}

func TestBuildCreateBucketRequest_RequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*mbOptions)
		want   string
	}{
		{"missing name", func(o *mbOptions) { o.Name = "" }, "missing bucket name"},
		{"missing project", func(o *mbOptions) { o.Project = "" }, "--project is required"},
		{"missing region", func(o *mbOptions) { o.Region = "" }, "--region <site> is required"},
		{"bad class", func(o *mbOptions) { o.StorageClass = "glacier" }, "invalid --storage-class"},
		{"bad retention mode", func(o *mbOptions) { o.Locking = true; o.RetentionMode = "FOREVER"; o.RetentionDays = 1 }, "invalid --retention-mode"},
		{"name with slash", func(o *mbOptions) { o.Name = "a/b" }, "cannot contain spaces or slashes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := baseMbOptions()
			tc.mutate(&o)
			_, err := buildCreateBucketRequest(o)
			if err == nil {
				t.Fatalf("expected an error")
			}
			if exitcode.Of(err) != exitcode.Usage {
				t.Errorf("exit code = %d, want %d (usage)", exitcode.Of(err), exitcode.Usage)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestBuildCreateBucketRequest_AWSRegionDetected(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-2", "ap-southeast-1"} {
		o := baseMbOptions()
		o.Region = region
		_, err := buildCreateBucketRequest(o)
		if err == nil {
			t.Fatalf("%s: expected an error", region)
		}
		if exitcode.Of(err) != exitcode.Usage {
			t.Errorf("%s: exit code = %d, want usage", region, exitcode.Of(err))
		}
		if !strings.Contains(err.Error(), "is not a Latitude site") || !strings.Contains(err.Error(), "DAL") {
			t.Errorf("%s: error %q should explain that regions are Latitude sites", region, err.Error())
		}
	}
	// Latitude slugs with digits are not mistaken for AWS regions.
	for _, region := range []string{"DAL", "TYO4", "SAO2", "nyc"} {
		o := baseMbOptions()
		o.Region = region
		if _, err := buildCreateBucketRequest(o); err != nil {
			t.Errorf("%s: unexpected error %v", region, err)
		}
	}
}

func TestBuildCreateBucketRequest_LockingImpliesVersioning(t *testing.T) {
	o := baseMbOptions()
	o.Locking = true
	req, err := buildCreateBucketRequest(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := req.Data.Attributes
	if a.Locking == nil || !*a.Locking {
		t.Errorf("locking should be true")
	}
	if a.Versioning == nil || !*a.Versioning {
		t.Errorf("versioning should be implied by --locking")
	}

	o.VersioningSet = true
	o.Versioning = false
	_, err = buildCreateBucketRequest(o)
	if err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("--locking --versioning=false should be a usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--locking requires versioning") {
		t.Errorf("unexpected message %q", err.Error())
	}
}

func TestBuildCreateBucketRequest_RetentionOnlyWithLocking(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*mbOptions)
	}{
		{"mode without locking", func(o *mbOptions) { o.RetentionMode = "GOVERNANCE"; o.RetentionDays = 7 }},
		{"days without locking", func(o *mbOptions) { o.RetentionDays = 7 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := baseMbOptions()
			tc.mutate(&o)
			_, err := buildCreateBucketRequest(o)
			if err == nil || exitcode.Of(err) != exitcode.Usage {
				t.Fatalf("expected a usage error, got %v", err)
			}
			if !strings.Contains(err.Error(), "require --locking") {
				t.Errorf("unexpected message %q", err.Error())
			}
		})
	}

	// Mode and days must come together.
	o := baseMbOptions()
	o.Locking = true
	o.RetentionMode = "compliance"
	if _, err := buildCreateBucketRequest(o); err == nil || !strings.Contains(err.Error(), "requires --retention-days") {
		t.Errorf("mode without days should be rejected, got %v", err)
	}
	o = baseMbOptions()
	o.Locking = true
	o.RetentionDays = 30
	if _, err := buildCreateBucketRequest(o); err == nil || !strings.Contains(err.Error(), "requires --retention-mode") {
		t.Errorf("days without mode should be rejected, got %v", err)
	}
	o = baseMbOptions()
	o.Locking = true
	o.RetentionMode = "GOVERNANCE"
	o.RetentionDays = -1
	if _, err := buildCreateBucketRequest(o); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Errorf("negative days should be rejected, got %v", err)
	}
}

func TestBuildCreateBucketRequest_RetentionMapsToPeriod(t *testing.T) {
	o := baseMbOptions()
	o.Locking = true
	o.RetentionMode = "compliance"
	o.RetentionDays = 30
	o.StorageClass = "high_performance"
	req, err := buildCreateBucketRequest(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := req.Data.Attributes
	if a.RetentionMode == nil || *a.RetentionMode != operations.RetentionModeCompliance {
		t.Errorf("retention mode = %v, want COMPLIANCE", a.RetentionMode)
	}
	if a.RetentionPeriod == nil || *a.RetentionPeriod != 30 {
		t.Errorf("retention period = %v, want 30", a.RetentionPeriod)
	}
	if a.StorageClass == nil || *a.StorageClass != operations.StorageClassHighPerformance {
		t.Errorf("storage class = %v, want high_performance", a.StorageClass)
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"retention_period":30`, `"retention_mode":"COMPLIANCE"`, `"locking":true`, `"versioning":true`, `"storage_class":"high_performance"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("request body %s lacks %s", body, want)
		}
	}
}

func TestDescribeCreateRequest(t *testing.T) {
	o := baseMbOptions()
	o.Locking = true
	o.RetentionMode = "GOVERNANCE"
	o.RetentionDays = 7
	req, err := buildCreateBucketRequest(o)
	if err != nil {
		t.Fatal(err)
	}
	got := describeCreateRequest(req.Data.Attributes)
	for _, want := range []string{"project: my-project", "region: DAL", "class: standard", "versioning: yes", "locking: GOVERNANCE (7d)"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q lacks %q", got, want)
		}
	}
}

func TestMbPlanJSON(t *testing.T) {
	req, err := buildCreateBucketRequest(baseMbOptions())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(mbPlan{Attributes: req.Data.Attributes, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["op"] != "make_bucket" || doc["dry_run"] != true || doc["name"] != "backups" || doc["region"] != "DAL" {
		t.Errorf("plan = %s", body)
	}
	if _, has := doc["retention_mode"]; has {
		t.Errorf("retention_mode should be omitted when NONE: %s", body)
	}
}

// Default key names and the clash policy now come from the shared helpers
// (defaultKeyName in accesskeys_shared.go, saveNewKey in s3.go), which are
// covered by their own tests; the mb-specific copies were removed.

func TestBuildCreateBucketRequest_StorageClassAliases(t *testing.T) {
	cases := map[string]operations.StorageClass{
		"":                 operations.StorageClassStandard,
		"standard":         operations.StorageClassStandard,
		"std":              operations.StorageClassStandard,
		"high_performance": operations.StorageClassHighPerformance,
		"high-performance": operations.StorageClassHighPerformance,
		"HP":               operations.StorageClassHighPerformance,
	}
	for in, want := range cases {
		o := baseMbOptions()
		o.StorageClass = in
		req, err := buildCreateBucketRequest(o)
		if err != nil {
			t.Errorf("%q: unexpected error %v", in, err)
			continue
		}
		if got := *req.Data.Attributes.StorageClass; got != want {
			t.Errorf("%q: storage class = %q, want %q", in, got, want)
		}
	}
}

func TestNewMbCmd_ProjectFlagRequiresRootPicker(t *testing.T) {
	cmd := NewMbCmd()
	if cmd.Flags().Lookup(flagProject) == nil {
		t.Fatalf("mb must register --project")
	}
	// mb needs exactly one project, so --project is NOT marked optional: the
	// root pre-run resolves it the same way as every other command (LSH_PROJECT,
	// then the interactive project picker in a terminal, else an error).
	if cmd.Annotations[cli.ProjectOptionalAnnotation] == "true" {
		t.Errorf("mb must not carry %s: the root pre-run should own the project prompt", cli.ProjectOptionalAnnotation)
	}
	// buildCreateBucketRequest still guards against an empty project as a safety net.
	o := baseMbOptions()
	o.Project = ""
	_, err := buildCreateBucketRequest(o)
	if err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "--project is required") {
		t.Errorf("missing project must stay a usage error, got %v", err)
	}
}

// mbUnsavedKeyFixture builds the state of a key that was created for a new
// bucket but could not be stored in the profile.
func mbUnsavedKeyFixture() (*createdAccessKey, accessKeyRequest, *objectstorage.Bucket) {
	created := &createdAccessKey{
		Name: "lsh-me-backups", AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "s3cr3t/value",
		Username:     "me+lsh-me-backups@latitude.sh",
		StorageClass: "standard", Project: "my-project", Scope: config.ScopeLimitedAccess,
	}
	req := accessKeyRequest{Project: "my-project", StorageClass: "standard", Name: "lsh-me-backups", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": config.PermissionRW}}
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "backups", BucketName: "backups-7f3a", Endpoint: "https://s3.us-central-1.storage.sh", SigningRegion: "us-central-1"}
	return created, req, b
}

// stubDiscard replaces the API deletion for the duration of a test.
func stubDiscard(t *testing.T, err error) *int {
	t.Helper()
	calls := 0
	saved := discardKey
	discardKey = func(context.Context, accessKeyRequest, *createdAccessKey) error {
		calls++
		return err
	}
	t.Cleanup(func() { discardKey = saved })
	return &calls
}

// TestMbReportUnsavedKey_RemovesInsteadOfPrinting covers the preferred
// recovery: a key that cannot be saved is deleted again, so no one-time secret
// is printed and no credential is left live on the API.
func TestMbReportUnsavedKey_RemovesInsteadOfPrinting(t *testing.T) {
	created, req, b := mbUnsavedKeyFixture()
	calls := stubDiscard(t, nil)

	var out bytes.Buffer
	mbReportUnsavedKey(context.Background(), &out, created, req, b, "lsh-me-backups", errors.New("read-only file system"))
	text := out.String()
	if *calls != 1 {
		t.Errorf("the key must be deleted exactly once, got %d calls", *calls)
	}
	if strings.Contains(text, "s3cr3t/value") {
		t.Errorf("no secret may be printed when the key was removed:\n%s", text)
	}
	for _, want := range []string{"could not be saved", "read-only file system", "It was deleted again", "Retry once the profile is writable"} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q:\n%s", want, text)
		}
	}
}

// TestMbReportUnsavedKey_PrintsSecretOnceWhenRemovalFails covers the fallback:
// the key is live and unusable unless its secret is shown, so it is printed —
// exactly once, with the command that stores it.
func TestMbReportUnsavedKey_PrintsSecretOnceWhenRemovalFails(t *testing.T) {
	created, req, b := mbUnsavedKeyFixture()
	stubDiscard(t, errors.New("403 forbidden"))

	var out bytes.Buffer
	mbReportUnsavedKey(context.Background(), &out, created, req, b, "lsh-me-backups", errors.New("read-only file system"))
	text := out.String()
	for _, want := range []string{
		"could not be saved", "read-only file system", "nor deleted again",
		"Access Key ID:", "AKIAEXAMPLE",
		"Secret Access Key:", "s3cr3t/value",
		"https://s3.us-central-1.storage.sh", "us-central-1",
		"backups-7f3a (rw)",
		"lsh s3 access-keys import --name lsh-me-backups --access-key-id AKIAEXAMPLE --project my-project --storage-class standard --bucket backups=rw",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "s3cr3t/value") != 1 {
		t.Errorf("the secret must be printed exactly once:\n%s", text)
	}
}
