package s3

import (
	"regexp"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

// TestConfigureDefaultNameIsPetName covers that configure uses the same
// pet-name generator as the dashboard for the default key name.
func TestConfigureDefaultNameIsPetName(t *testing.T) {
	re := regexp.MustCompile(`^key-[a-z]+-[a-z]+-(std|hp)$`)
	if got := generateKeyName(objectstorage.ClassHighPerformance); !re.MatchString(got) {
		t.Errorf("generateKeyName(hp) = %q", got)
	}
}

// TestConfigureBucketSpecsUseSharedParser checks the --bucket spellings the
// configure help advertises through the parser shared with access-keys
// create (configure used to have its own copy).
func TestConfigureBucketSpecsUseSharedParser(t *testing.T) {
	cases := map[string]bucketSpec{
		"backups":              {Token: "backups", Permission: config.PermissionRW},
		"backups=rw":           {Token: "backups", Permission: config.PermissionRW},
		"logs=readonly":        {Token: "logs", Permission: config.PermissionReadOnly},
		" logs = RO ":          {Token: "logs", Permission: config.PermissionReadOnly},
		"s3://bkt_1Gjb=rw":     {Token: "bkt_1Gjb", Permission: config.PermissionRW},
		"s3://backups/":        {Token: "backups", Permission: config.PermissionRW},
		"backups-7f3a=read":    {Token: "backups-7f3a", Permission: config.PermissionReadOnly},
		"backups=read-write":   {Token: "backups", Permission: config.PermissionRW},
		"bkt_9aQ2xL0mPq7Rt=ro": {Token: "bkt_9aQ2xL0mPq7Rt", Permission: config.PermissionReadOnly},
	}
	for in, want := range cases {
		specs, err := parseBucketSpecs([]string{in})
		if err != nil {
			t.Errorf("parseBucketSpecs(%q): %v", in, err)
			continue
		}
		if len(specs) != 1 || specs[0] != want {
			t.Errorf("parseBucketSpecs(%q) = %+v, want %+v", in, specs, want)
		}
	}
	for _, bad := range []string{"", "=rw", "backups=admin", "s3://"} {
		if _, err := parseBucketSpec(bad); err == nil || exitcode.Of(err) != exitcode.Usage {
			t.Errorf("parseBucketSpec(%q) must fail with exit %d, got %v", bad, exitcode.Usage, err)
		}
	}
	if _, err := parseBucketSpecs([]string{"backups", "backups=readonly"}); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("duplicate bucket must be a usage error, got %v", err)
	}
	if specs, err := parseBucketSpecs(nil); err != nil || len(specs) != 0 {
		t.Fatalf("empty input: %v %v", specs, err)
	}
}

func TestCfgValidateSelection(t *testing.T) {
	std := func(id, name, project string) *objectstorage.Bucket {
		return &objectstorage.Bucket{ID: id, Name: name, BucketName: name + "-1", StorageClass: objectstorage.ClassStandard, ProjectID: project, Site: "DAL"}
	}
	hp := func(id, name, site, project string) *objectstorage.Bucket {
		return &objectstorage.Bucket{ID: id, Name: name, BucketName: name + "-1", StorageClass: objectstorage.ClassHighPerformance, ProjectID: project, Site: site}
	}

	class, site, project, err := cfgValidateSelection([]cfgSelectedBucket{
		{Bucket: std("bkt_1", "backups", "proj_1"), Permission: config.PermissionRW},
		{Bucket: std("bkt_2", "logs", "proj_1"), Permission: config.PermissionReadOnly},
	})
	if err != nil || class != objectstorage.ClassStandard || site != "DAL" || project != "proj_1" {
		t.Fatalf("same group must pass: %q %q %q %v", class, site, project, err)
	}

	// Standard buckets on different sites are fine (one key covers every site).
	b2 := std("bkt_2", "logs", "proj_1")
	b2.Site = "NYC"
	if _, _, _, err := cfgValidateSelection([]cfgSelectedBucket{{Bucket: std("bkt_1", "backups", "proj_1")}, {Bucket: b2}}); err != nil {
		t.Fatalf("standard buckets may span sites: %v", err)
	}

	// Empty site on one side is treated as unknown, not as a conflict.
	unknownSite := hp("bkt_3", "fast", "", "proj_1")
	_, site, _, err = cfgValidateSelection([]cfgSelectedBucket{{Bucket: hp("bkt_4", "faster", "TYO4", "proj_1")}, {Bucket: unknownSite}})
	if err != nil || site != "TYO4" {
		t.Fatalf("unknown site must not conflict and the known one wins: %q %v", site, err)
	}

	failures := []struct {
		name string
		sel  []cfgSelectedBucket
		want string
	}{
		{"class", []cfgSelectedBucket{{Bucket: std("bkt_1", "backups", "proj_1")}, {Bucket: hp("bkt_3", "fast", "DAL", "proj_1")}}, "storage classes"},
		{"site", []cfgSelectedBucket{{Bucket: hp("bkt_3", "fast", "TYO4", "proj_1")}, {Bucket: hp("bkt_4", "faster", "DAL", "proj_1")}}, "different sites"},
		{"project", []cfgSelectedBucket{{Bucket: std("bkt_1", "backups", "proj_1")}, {Bucket: std("bkt_2", "logs", "proj_2")}}, "different projects"},
		{"empty", nil, "no bucket"},
	}
	for _, f := range failures {
		_, _, _, err := cfgValidateSelection(f.sel)
		if err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), f.want) {
			t.Errorf("%s: want usage error containing %q, got %v", f.name, f.want, err)
		}
	}
}

// TestConfigureSiteFromBuckets covers the site inference of a fullaccess key
// (F16): a standard key is never created without a site; the site comes from
// the project's buckets of the key's class, the raw site map filling gaps.
func TestConfigureSiteFromBuckets(t *testing.T) {
	std := func(id, site string) *objectstorage.Bucket {
		return &objectstorage.Bucket{ID: id, Name: id, StorageClass: objectstorage.ClassStandard, Site: site}
	}
	hp := func(id, site string) *objectstorage.Bucket {
		return &objectstorage.Bucket{ID: id, Name: id, StorageClass: objectstorage.ClassHighPerformance, Site: site}
	}

	// Standard: any site of a standard bucket is valid; the choice is
	// deterministic (sorted) and other classes are ignored.
	site, err := configureSiteFromBuckets(objectstorage.ClassStandard, "proj_1",
		[]*objectstorage.Bucket{std("bkt_1", "nyc"), std("bkt_2", "DAL"), hp("bkt_3", "TYO4")}, nil)
	if err != nil || site != "DAL" {
		t.Fatalf("standard site: %q %v", site, err)
	}
	// The raw site map fills buckets whose SDK model lost the site.
	site, err = configureSiteFromBuckets(objectstorage.ClassStandard, "proj_1",
		[]*objectstorage.Bucket{std("bkt_1", "")}, map[string]string{"bkt_1": "sao"})
	if err != nil || site != "SAO" {
		t.Fatalf("site from raw map: %q %v", site, err)
	}
	// Nothing of the class → "" (the caller asks or fails with exit 2).
	site, err = configureSiteFromBuckets(objectstorage.ClassStandard, "proj_1", []*objectstorage.Bucket{hp("bkt_3", "TYO4")}, nil)
	if err != nil || site != "" {
		t.Fatalf("no standard buckets must yield an empty site: %q %v", site, err)
	}
	site, err = configureSiteFromBuckets(objectstorage.ClassStandard, "proj_1", nil, nil)
	if err != nil || site != "" {
		t.Fatalf("no buckets at all: %q %v", site, err)
	}
	// high_performance: one site is fine, several are ambiguous.
	site, err = configureSiteFromBuckets(objectstorage.ClassHighPerformance, "proj_1", []*objectstorage.Bucket{hp("bkt_3", "TYO4"), hp("bkt_4", "tyo4")}, nil)
	if err != nil || site != "TYO4" {
		t.Fatalf("single hp site: %q %v", site, err)
	}
	_, err = configureSiteFromBuckets(objectstorage.ClassHighPerformance, "proj_1", []*objectstorage.Bucket{hp("bkt_3", "TYO4"), hp("bkt_4", "DAL")}, nil)
	if err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "DAL, TYO4") || !strings.Contains(err.Error(), "--region") {
		t.Fatalf("several hp sites must be a usage error naming them and --region, got %v", err)
	}
}

func TestConfigurePlanLineAndBuckets(t *testing.T) {
	full := configurePlan{Name: "lsh-lanusse-tyo4", Project: "my-project", StorageClass: objectstorage.ClassHighPerformance, Site: "TYO4", Scope: config.ScopeFullAccess, Save: true}
	want := `(dryrun) create access key "lsh-lanusse-tyo4" in project my-project for all high_performance buckets in TYO4 (fullaccess) and save it in the active profile`
	if got := configurePlanLine(full); got != want {
		t.Fatalf("fullaccess plan line:\n got %s\nwant %s", got, want)
	}
	// A standard fullaccess key keeps its site too (it is no longer cleared).
	std := configurePlan{Name: "lsh-lanusse-standard", Project: "my-project", StorageClass: objectstorage.ClassStandard, Site: "DAL", Scope: config.ScopeFullAccess, Save: true}
	if got := configurePlanLine(std); !strings.Contains(got, "all standard buckets in DAL (fullaccess)") {
		t.Fatalf("standard plan line must name the site: %s", got)
	}
	limited := configurePlan{Name: "ci", Project: "proj_1", StorageClass: objectstorage.ClassStandard, Scope: config.ScopeLimitedAccess,
		Buckets: map[string]string{"logs": config.PermissionReadOnly, "backups": config.PermissionRW}, Save: false}
	want = `(dryrun) create access key "ci" in project proj_1 for buckets backups=rw logs=readonly (limited_access) and print the secret once (not saved)`
	if got := configurePlanLine(limited); got != want {
		t.Fatalf("limited plan line:\n got %s\nwant %s", got, want)
	}
	// Empty cells use the placeholder shared by the whole group.
	if got := cfgFormatBucketPerms(nil); got != emptyCell {
		t.Fatalf("empty perms placeholder: %q", got)
	}
	row := limited.TableRow()
	if row["buckets"].Value != "backups=rw logs=readonly" || row["status"].Value != "created" || row["save"].Value != "no" || row["site"].Value != emptyCell {
		t.Fatalf("table row: %+v", row)
	}
	if _, ok := row["saved_as"]; ok {
		t.Fatal("saved_as cell only when the key was stored")
	}
	saved := full
	saved.SavedAs, saved.Profile = "lsh-lanusse-tyo4-2", "default"
	if got := saved.TableRow()["saved_as"].Value; got != "lsh-lanusse-tyo4-2" {
		t.Fatalf("saved_as cell: %q", got)
	}
}

func TestConfigureStatePerms(t *testing.T) {
	st := &configureState{Scope: config.ScopeLimitedAccess, Selected: []cfgSelectedBucket{
		{Bucket: &objectstorage.Bucket{ID: "bkt_1", Name: "backups"}, Permission: config.PermissionRW},
		{Bucket: &objectstorage.Bucket{ID: "bkt_2", Name: "logs"}, Permission: config.PermissionReadOnly},
	}}
	if p := st.bucketPerms(); p["bkt_1"] != config.PermissionRW || p["bkt_2"] != config.PermissionReadOnly || len(p) != 2 {
		t.Fatalf("bucketPerms by id: %v", p)
	}
	if p := st.bucketNamePerms(); p["backups"] != config.PermissionRW || p["logs"] != config.PermissionReadOnly {
		t.Fatalf("bucketNamePerms by name: %v", p)
	}
	st.Scope = config.ScopeFullAccess
	if st.bucketPerms() != nil || st.bucketNamePerms() != nil {
		t.Fatal("fullaccess keys carry no bucket map")
	}
	if (&configureState{ProjectToken: "my-project", ProjectID: "proj_1"}).project() != "my-project" || (&configureState{ProjectID: "proj_1"}).project() != "proj_1" {
		t.Fatal("project() prefers the token as given, then the resolved ID")
	}
}

func TestConfigureNeedsPromptIsUsageError(t *testing.T) {
	err := configureNeedsPrompt("the project")
	if exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("exit code %d, want %d", exitcode.Of(err), exitcode.Usage)
	}
	msg := err.Error()
	for _, want := range []string{"the project", "lsh s3 access-keys create --bucket <b> --save", objectstorage.EnvAccessKeyID, objectstorage.EnvSecretAccessKey} {
		if !strings.Contains(msg, want) {
			t.Errorf("message must mention %q: %s", want, msg)
		}
	}
}

// TestConfigureFlagsParse exercises the flag parsing that now goes through
// the shared helpers (ParseStorageClass, regionFlag, parseBucketSpecs).
func TestConfigureFlagsParse(t *testing.T) {
	parse := func(args ...string) (configureOptions, error) {
		cmd := NewConfigureCmd()
		cmd.SetArgs(args)
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("ParseFlags(%v): %v", args, err)
		}
		return parseConfigureOptions(cmd)
	}
	o, err := parse("--storage-class", "high-performance", "--region", "tyo4", "--bucket", "backups=rw", "--bucket", "logs=ro", "--name", " laptop ")
	if err != nil {
		t.Fatal(err)
	}
	if o.StorageClass != objectstorage.ClassHighPerformance || o.Site != "TYO4" || o.Name != "laptop" || !o.Save {
		t.Fatalf("options: %+v", o)
	}
	if len(o.Buckets) != 2 || o.Buckets[1].Token != "logs" || o.Buckets[1].Permission != config.PermissionReadOnly {
		t.Fatalf("bucket specs: %+v", o.Buckets)
	}
	if o, err := parse("--save=false"); err != nil || o.Save {
		t.Fatalf("--save=false: %+v %v", o, err)
	}
	for _, bad := range [][]string{
		{"--storage-class", "glacier"},
		{"--region", "us-east-1"},
		{"--bucket", "backups=admin"},
		{"--all-buckets", "--bucket", "backups"},
	} {
		if _, err := parse(bad...); err == nil || exitcode.Of(err) != exitcode.Usage {
			t.Errorf("%v must be a usage error, got %v", bad, err)
		}
	}
}
