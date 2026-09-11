package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

func TestCreateStoredRoundTrip(t *testing.T) {
	home := withTempProfile(t, nil)
	f := newFakeAPI(t)
	api := f.client()

	req := accessKeyRequest{
		Project: "proj_1", StorageClass: objectstorage.ClassStandard, Site: "DAL", Name: "CI Deploy",
		Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": "rw", "bkt_2": "readonly"},
	}
	created, err := api.create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if created.AccessKeyID != "WASABIKEYID" || created.SecretAccessKey != "wasabi-secret-value" || created.Name != "ci deploy" {
		t.Fatalf("unexpected created key: id=%s name=%s", created.AccessKeyID, created.Name)
	}

	// The request body carries the expected JSON:API shape.
	reqs := f.Requests()
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/storage/access_keys" {
		t.Fatalf("unexpected requests: %+v", reqs)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(reqs[0].Body), &body); err != nil {
		t.Fatal(err)
	}
	attrs := body["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if attrs["access_scope"] != "limited_access" || attrs["region"] != "DAL" || attrs["storage_class"] != "standard" || attrs["project"] != "proj_1" {
		t.Fatalf("unexpected attributes: %v", attrs)
	}
	perms := attrs["bucket_permissions"].([]interface{})
	if len(perms) != 2 || perms[0].(map[string]interface{})["bucket_id"] != "bkt_1" {
		t.Fatalf("unexpected bucket_permissions: %v", perms)
	}

	// Save into the temp profile and read it back through the same helpers.
	stored := created.stored("proj_1", req.Buckets, config.KeySourceCreate)
	profileName, err := objectstorage.SaveKey("", "ci-deploy", stored)
	if err != nil || profileName != "test" {
		t.Fatalf("SaveKey: profile=%q err=%v", profileName, err)
	}
	_, _, p, err := objectstorage.ActiveProfile("")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.ObjectStorageKeys()["ci-deploy"]
	if !ok {
		t.Fatalf("key not saved: %v", p.ObjectStorageKeys())
	}
	if got.AccessKeyID != "WASABIKEYID" || got.SecretAccessKey != "wasabi-secret-value" || got.Scope != config.ScopeLimitedAccess ||
		got.Buckets["bkt_2"] != "readonly" || got.Username != "someone+ci deploy@example.com" || got.Source != config.KeySourceCreate ||
		got.StorageClass != "standard" || got.ProjectID != "proj_1" || got.CreatedAt.IsZero() {
		t.Fatalf("stored key mismatch: %s scope=%s buckets=%v", got, got.Scope, got.Buckets)
	}
	raw := readConfigRaw(t, home)
	if !strings.Contains(raw, `"secret_access_key": "wasabi-secret-value"`) || !strings.Contains(raw, `"scope": "limited_access"`) {
		t.Fatalf("config.json content unexpected:\n%s", raw)
	}

	// The credential resolver now picks the key for the bucket it covers.
	cred, err := objectstorage.ResolveCredential(&objectstorage.Bucket{ID: "bkt_1", Name: "backups", BucketName: "backups-7f3a", Endpoint: "https://s3.us-central-1.storage.sh", StorageClass: "standard", ProjectID: "proj_1"}, objectstorage.CredentialOptions{Write: true})
	if err != nil || cred.Name != "ci-deploy" || cred.Secret() != "wasabi-secret-value" {
		t.Fatalf("saved key not selected: %v err=%v", cred, err)
	}

	// ForgetKeyByID removes it again.
	_, removed, err := objectstorage.ForgetKeyByID("", "WASABIKEYID")
	if err != nil || len(removed) != 1 || removed[0] != "ci-deploy" {
		t.Fatalf("ForgetKeyByID: %v err=%v", removed, err)
	}
	if strings.Contains(readConfigRaw(t, home), "wasabi-secret-value") {
		t.Fatal("secret still on disk after forget")
	}
}

func TestCreateVASTShapeAndAPIErrors(t *testing.T) {
	f := newFakeAPI(t)
	f.createShape = "vast"
	api := f.client()
	req := accessKeyRequest{Project: "proj_1", StorageClass: objectstorage.ClassHighPerformance, Site: "TYO4", Name: "fast", Scope: config.ScopeFullAccess}
	created, err := api.create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if created.AccessKeyID != "VASTKEYID" || created.SecretAccessKey != "vast-secret-value" || created.Site != "TYO4" || created.Buckets != nil {
		t.Fatalf("unexpected created key: %+v", created.stored("", nil, ""))
	}

	// Validation errors never reach the API.
	before := len(f.Requests())
	bad := req
	bad.Site = ""
	if _, err := api.create(context.Background(), bad); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("missing site must be a usage error, got %v", err)
	}
	if len(f.Requests()) != before {
		t.Fatal("invalid request must not hit the API")
	}
}

func TestCreatedKeyViewSecretHandling(t *testing.T) {
	k := &createdAccessKey{
		Name: "ci-deploy", AccessKeyID: "XL68DDURVGUUOULWPCAE", SecretAccessKey: "the-secret-value", StorageClass: "standard", Site: "DAL",
		Project: "my-project", Scope: config.ScopeLimitedAccess, Endpoint: "https://s3.us-central-1.storage.sh", SigningRegion: "us-central-1",
	}
	plan := &createPlan{Buckets: []scopedBucket{
		{Bucket: &objectstorage.Bucket{ID: "bkt_1", Name: "backups", BucketName: "backups-7f3a"}, Permission: "rw"},
		{Bucket: &objectstorage.Bucket{ID: "bkt_2", Name: "logs", BucketName: "logs-91aa"}, Permission: "readonly"},
	}}

	// Shown once, in clear, with the J3 layout.
	var buf bytes.Buffer
	printCreatedHuman(&buf, newCreatedKeyView(k, plan, true), true)
	out := buf.String()
	for _, want := range []string{
		`Access key "ci-deploy" created. The secret is shown once and cannot be retrieved again:`,
		"Access Key ID:      XL68DDURVGUUOULWPCAE",
		"Secret Access Key:  the-secret-value",
		"Endpoint:           https://s3.us-central-1.storage.sh",
		"Signing region:     us-central-1",
		"Site:               DAL",
		"Buckets:            backups-7f3a (rw), logs-91aa (readonly)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}

	// Omitted: no secret at all, not even a fragment.
	buf.Reset()
	v := newCreatedKeyView(k, plan, false)
	printCreatedHuman(&buf, v, false)
	if strings.Contains(buf.String(), "secret") || strings.Contains(buf.String(), "the-") || strings.Contains(buf.String(), "value") {
		t.Fatalf("secret (or a fragment) leaked:\n%s", buf.String())
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "secret") {
		t.Fatalf("JSON must omit secret_access_key entirely: %s", b)
	}
	if !strings.Contains(string(b), `"buckets":[{"bucket_name":"backups-7f3a","name":"backups","id":"bkt_1","permission":"rw"}`) {
		t.Fatalf("JSON buckets shape unexpected: %s", b)
	}
	b, _ = json.Marshal(newCreatedKeyView(k, plan, true))
	if !strings.Contains(string(b), `"secret_access_key":"the-secret-value"`) || !strings.Contains(string(b), `"signing_region":"us-central-1"`) {
		t.Fatalf("explicit JSON must include the secret and endpoint data: %s", b)
	}

	// Fullaccess keys describe their coverage instead of listing buckets.
	full := *k
	full.Scope = config.ScopeFullAccess
	fv := newCreatedKeyView(&full, &createPlan{}, false)
	if got := fv.bucketsLine(); got != "all standard buckets of project my-project in DAL" {
		t.Fatalf("fullaccess buckets line = %q", got)
	}
}

func TestApplyLike(t *testing.T) {
	like := config.StoredAccessKey{StorageClass: "standard", Site: "", ProjectID: "proj_1", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": "rw", "bkt_2": "readonly"}}
	o := createOptions{Like: "ci", BucketSpecs: []bucketSpec{{Token: "bkt_3", Permission: "rw"}}}
	if err := applyLike(&o, like); err != nil {
		t.Fatal(err)
	}
	if o.Project != "proj_1" || o.StorageClass != "standard" || len(o.BucketSpecs) != 3 || o.BucketSpecs[1].Token != "bkt_1" || o.BucketSpecs[2].Permission != "readonly" {
		t.Fatalf("applyLike merged wrongly: %+v", o)
	}
	full := config.StoredAccessKey{StorageClass: "high_performance", Site: "tyo4", ProjectID: "proj_1", Scope: config.ScopeFullAccess}
	o = createOptions{Like: "hp"}
	if err := applyLike(&o, full); err != nil || !o.AllBuckets || o.Site != "TYO4" {
		t.Fatalf("fullaccess template must set --all-buckets and the site: %+v err=%v", o, err)
	}
	if err := applyLike(&createOptions{Like: "u"}, config.StoredAccessKey{Scope: config.ScopeUnknown}); err == nil {
		t.Fatal("unknown scope template must fail")
	}
}

func TestBuildCreatePlanScopeErrors(t *testing.T) {
	r := &objectstorage.Resolver{}
	if _, err := buildCreatePlan(context.Background(), r, createOptions{}); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("no scope must be a usage error, got %v", err)
	}
	if _, err := buildCreatePlan(context.Background(), r, createOptions{AllBuckets: true, BucketSpecs: []bucketSpec{{Token: "a"}}}); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("both scopes must be a usage error, got %v", err)
	}
	if _, err := buildCreatePlan(context.Background(), r, createOptions{AllBuckets: true, StorageClass: "standard"}); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("--all-buckets without project must ask for --project, got %v", err)
	}
}

// TestParseCreateOptionsSaveFlags covers F27: --save is a plain bool,
// --save-as <name> implies --save, and --save=false never saves.
func TestParseCreateOptionsSaveFlags(t *testing.T) {
	parse := func(args ...string) createOptions {
		t.Helper()
		cmd := newAccessKeysCreateCmd()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("ParseFlags(%v): %v", args, err)
		}
		o, err := parseCreateOptions(cmd)
		if err != nil {
			t.Fatalf("parseCreateOptions(%v): %v", args, err)
		}
		return o
	}
	if o := parse("--bucket", "backups"); o.Save || o.SaveName != "" {
		t.Fatalf("default must not save: %+v", o)
	}
	if o := parse("--bucket", "backups", "--save"); !o.Save || o.SaveName != "" {
		t.Fatalf("--save must save under the key name: %+v", o)
	}
	if o := parse("--bucket", "backups", "--save=false"); o.Save {
		t.Fatalf("--save=false must not save: %+v", o)
	}
	if o := parse("--bucket", "backups", "--save-as", "ops"); !o.Save || o.SaveName != "ops" {
		t.Fatalf("--save-as must imply --save with that name: %+v", o)
	}
	if o := parse("--bucket", "backups", "--save", "--save-as", " ops "); !o.Save || o.SaveName != "ops" {
		t.Fatalf("--save --save-as must trim the name: %+v", o)
	}
	cmd := newAccessKeysCreateCmd()
	if err := cmd.ParseFlags([]string{"--save=ops"}); err == nil {
		t.Fatal("--save=<name> is no longer accepted; --save-as replaces it")
	}
}

func TestGenerateKeyName(t *testing.T) {
	re := regexp.MustCompile(`^key-[a-z]+-[a-z]+-(std|hp)$`)
	for _, class := range []string{"standard", "high_performance", ""} {
		got := generateKeyName(class)
		if !re.MatchString(got) {
			t.Errorf("generateKeyName(%q) = %q, want key-<adj>-<noun>-<std|hp>", class, got)
		}
		if len(got) < 3 || len(got) > maxAccessKeyNameLen {
			t.Errorf("generateKeyName(%q) = %q, length %d out of 3..%d", class, got, len(got), maxAccessKeyNameLen)
		}
		if err := validateAccessKeyName(got); err != nil {
			t.Errorf("generated name %q rejected by validator: %v", got, err)
		}
	}
	if tierAbbr("high_performance") != "hp" || tierAbbr("standard") != "std" || tierAbbr("") != "std" {
		t.Error("tierAbbr mapping wrong")
	}
}
