package objectstorage

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func TestEnvCredential(t *testing.T) {
	t.Setenv(EnvAccessKeyID, "")
	t.Setenv(EnvSecretAccessKey, "")
	t.Setenv(EnvUseAWSEnv, "")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAREAL")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "realsecret")
	t.Setenv("AWS_SESSION_TOKEN", "")

	if _, ok, err := EnvCredential(); ok || err != nil {
		t.Fatalf("AWS_* must be ignored without opt-in: ok=%v err=%v", ok, err)
	}

	t.Setenv(EnvUseAWSEnv, "1")
	c, ok, err := EnvCredential()
	if !ok || err != nil || c.AccessKeyID != "AKIAREAL" || c.Secret() != "realsecret" || !c.FromEnv {
		t.Fatalf("opt-in fallback failed: %+v ok=%v err=%v", c, ok, err)
	}
	t.Setenv("AWS_SESSION_TOKEN", "sts")
	if _, ok, _ := EnvCredential(); ok {
		t.Fatal("STS credentials must never be used as Latitude keys")
	}

	t.Setenv(EnvAccessKeyID, "LSHID")
	if _, _, err := EnvCredential(); err == nil || exitcode.Of(err) != exitcode.Credentials {
		t.Fatalf("half-set pair must fail with exit %d, got %v", exitcode.Credentials, err)
	}
	t.Setenv(EnvSecretAccessKey, "LSHSECRET")
	c, ok, err = EnvCredential()
	if !ok || err != nil || c.AccessKeyID != "LSHID" || c.Secret() != "LSHSECRET" {
		t.Fatalf("LSH_S3_* must win: %+v", c)
	}
	if strings.Contains(c.String(), "LSHSECRET") {
		t.Fatal("String() leaked the secret")
	}
}

func TestSelectKeyLeastPrivilege(t *testing.T) {
	b := &Bucket{ID: "bkt_1", StorageClass: config.ScopeFullAccess, Site: "DAL", ProjectID: "proj_1"}
	b.StorageClass = "standard"
	now := time.Now()
	keys := map[string]config.StoredAccessKey{
		"full-std": {AccessKeyID: "F", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeFullAccess, CreatedAt: now.Add(-time.Hour)},
		"ro-1":     {AccessKeyID: "R", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": config.PermissionReadOnly}, CreatedAt: now},
		"rw-1":     {AccessKeyID: "W", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_1": config.PermissionRW}, CreatedAt: now.Add(-2 * time.Hour)},
		"other":    {AccessKeyID: "O", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeLimitedAccess, Buckets: map[string]string{"bkt_9": config.PermissionRW}, CreatedAt: now},
		"hp":       {AccessKeyID: "H", StorageClass: "high_performance", Site: "DAL", ProjectID: "proj_1", Scope: config.ScopeFullAccess, CreatedAt: now},
		"unknown":  {AccessKeyID: "U", StorageClass: "standard", Scope: config.ScopeUnknown, CreatedAt: now},
	}

	name, _, ok := SelectKey(keys, "", b, false)
	if !ok || name != "rw-1" {
		t.Fatalf("read: want rw-1 (limited rw ranks first), got %q ok=%v", name, ok)
	}
	name, _, ok = SelectKey(keys, "", b, true)
	if !ok || name != "rw-1" {
		t.Fatalf("write: want rw-1, got %q", name)
	}

	delete(keys, "rw-1")
	name, _, _ = SelectKey(keys, "", b, false)
	if name != "ro-1" {
		t.Fatalf("read without rw key: want ro-1 over fullaccess, got %q", name)
	}
	name, _, _ = SelectKey(keys, "", b, true)
	if name != "full-std" {
		t.Fatalf("write must skip readonly and use fullaccess, got %q", name)
	}

	delete(keys, "full-std")
	if _, _, ok := SelectKey(keys, "", b, true); ok {
		t.Fatal("no writable key must yield no selection")
	}

	// Fullaccess ties: default key wins, then newest.
	keys = map[string]config.StoredAccessKey{
		"old":  {AccessKeyID: "A", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeFullAccess, CreatedAt: now.Add(-time.Hour)},
		"new":  {AccessKeyID: "B", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeFullAccess, CreatedAt: now},
		"dflt": {AccessKeyID: "C", StorageClass: "standard", ProjectID: "proj_1", Scope: config.ScopeFullAccess, CreatedAt: now.Add(-2 * time.Hour)},
	}
	if name, _, _ := SelectKey(keys, "dflt", b, false); name != "dflt" {
		t.Fatalf("default key must break ties, got %q", name)
	}
	if name, _, _ := SelectKey(keys, "", b, false); name != "new" {
		t.Fatalf("newest key must break ties, got %q", name)
	}

	// A high_performance bucket in another site is not covered by a DAL key.
	hp := &Bucket{ID: "bkt_2", StorageClass: "high_performance", Site: "NYC", ProjectID: "proj_1"}
	keys = map[string]config.StoredAccessKey{"hp-dal": {AccessKeyID: "H", StorageClass: "high_performance", Site: "DAL", ProjectID: "proj_1", Scope: config.ScopeFullAccess}}
	if _, _, ok := SelectKey(keys, "", hp, false); ok {
		t.Fatal("DAL key must not cover NYC high_performance bucket")
	}
	hp.Site = "dal"
	if _, _, ok := SelectKey(keys, "", hp, false); !ok {
		t.Fatal("site comparison must be case-insensitive")
	}
}

func TestResolveCredentialEndpointOverrideRequiresEnv(t *testing.T) {
	t.Setenv(EnvAccessKeyID, "")
	t.Setenv(EnvSecretAccessKey, "")
	t.Setenv(EnvUseAWSEnv, "")
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	b := &Bucket{BucketName: "raw", Endpoint: "https://s3.example.test", EndpointOverride: true}
	_, err := ResolveCredential(b, CredentialOptions{})
	if err == nil || exitcode.Of(err) != exitcode.Credentials {
		t.Fatalf("saved keys must not be used with --endpoint-url; got %v", err)
	}
	if !strings.Contains(err.Error(), EnvAccessKeyID) {
		t.Errorf("error should tell the user which variables to set: %v", err)
	}
}

// TestExplicitKeyChecksCompatibility covers the --access-key path: Covers() is
// true for every bucket on a fullaccess key, so without an explicit
// compatibility check a key from another backend, site or project would be
// sent to the bucket and fail at the server instead of locally.
func TestExplicitKeyChecksCompatibility(t *testing.T) {
	hp := &Bucket{ID: "bkt_1", Name: "fast", StorageClass: ClassHighPerformance, Site: "TYO4", ProjectID: "proj_1"}
	full := func(class, site, project string) config.StoredAccessKey {
		return config.StoredAccessKey{AccessKeyID: "AK", StorageClass: class, Site: site, ProjectID: project, Scope: config.ScopeFullAccess}
	}
	cases := []struct {
		name string
		key  config.StoredAccessKey
		want string
	}{
		{"other storage class", full(ClassStandard, "TYO4", "proj_1"), "do not share credentials"},
		{"other site", full(ClassHighPerformance, "DAL", "proj_1"), "only works in its own site"},
		{"other project", full(ClassHighPerformance, "TYO4", "proj_2"), "belongs to project"},
	}
	for _, c := range cases {
		err := explainIncompatibleKey("ci-key", c.key, hp)
		if err == nil {
			t.Errorf("%s: expected the selection to be rejected", c.name)
			continue
		}
		if code := exitcode.Of(err); code != exitcode.Usage {
			t.Errorf("%s: exit code = %d, want %d", c.name, code, exitcode.Usage)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q should explain the mismatch (%q)", c.name, err, c.want)
		}
	}

	// A compatible key passes, and partial metadata is not judged: an imported
	// key with no class/site/project stays usable.
	for _, ok := range []config.StoredAccessKey{
		full(ClassHighPerformance, "tyo4", "proj_1"),
		full("", "", ""),
		{AccessKeyID: "AK", Scope: config.ScopeUnknown},
	} {
		if err := explainIncompatibleKey("ci-key", ok, hp); err != nil {
			t.Errorf("key %+v must be accepted, got %v", ok, err)
		}
	}
}
