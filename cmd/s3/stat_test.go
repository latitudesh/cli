package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
	"github.com/minio/minio-go/v7"
)

func TestStatObject_ReturnsMetadata(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	obj := srv.AddObject("backups-7f3a", "2026/09/dump.sql", []byte("select 1;"), "application/sql")
	obj.Metadata = map[string]string{"owner": "ops", "env": "prod"}
	sess := newRbStatSession(t, srv, "backups-7f3a")

	got, err := statObject(context.Background(), sess, "2026/09/dump.sql", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Key != "2026/09/dump.sql" {
		t.Errorf("key = %q", got.Key)
	}
	if got.Size != int64(len("select 1;")) {
		t.Errorf("size = %d", got.Size)
	}
	if got.ContentType != "application/sql" {
		t.Errorf("content type = %q", got.ContentType)
	}
	if got.ETag == "" || strings.Contains(got.ETag, `"`) {
		t.Errorf("etag should be set and unquoted, got %q", got.ETag)
	}
	if got.LastModified.IsZero() {
		t.Errorf("last modified should be set")
	}
	if got.Metadata["owner"] != "ops" || got.Metadata["env"] != "prod" {
		t.Errorf("metadata = %v", got.Metadata)
	}
	if got.VersionID != "" {
		t.Errorf("unversioned object should have no version id, got %q", got.VersionID)
	}
	if got.IsLatest != nil {
		t.Errorf("a HEAD response carries no is_latest flag, got %v", *got.IsLatest)
	}

	// Only a HEAD reaches the backend.
	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Method != http.MethodHead {
		t.Errorf("expected a single HEAD, got %+v", reqs)
	}

	var out bytes.Buffer
	writeObjectStat(&out, got)
	text := out.String()
	for _, want := range []string{"Key:", "2026/09/dump.sql", "Size:", "9 (9 Bytes)", "Content type:", "application/sql", "Metadata:", "env=prod owner=ops"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

func TestStatObject_VersionID(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	bkt := srv.CreateBucket("backups-7f3a")
	bkt.Versioned = true
	// Pin distinct version IDs (the fake server would reuse v1 for both).
	srv.AddObject("backups-7f3a", "a.txt", []byte("one"), "text/plain").VersionID = "v1"
	srv.AddObject("backups-7f3a", "a.txt", []byte("three"), "text/plain").VersionID = "v2"
	sess := newRbStatSession(t, srv, "backups-7f3a")

	got, err := statObject(context.Background(), sess, "a.txt", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Size != 3 || got.VersionID != "v1" {
		t.Errorf("got size=%d version=%q, want the old version", got.Size, got.VersionID)
	}
	// HEAD never reports whether a version is the latest (only listings do), so
	// stat must not fabricate is_latest even when --version-id was given.
	if got.IsLatest != nil {
		t.Errorf("is_latest must stay unset for stat, got %v", *got.IsLatest)
	}
	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Query.Get("versionId") != "v1" {
		t.Errorf("HEAD should carry versionId=v1, got %+v", reqs)
	}
}

func TestStatObject_NotFoundIsExit3(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	sess := newRbStatSession(t, srv, "backups-7f3a")

	_, err := statObject(context.Background(), sess, "missing.txt", "")
	if err == nil {
		t.Fatalf("expected an error")
	}
	if exitcode.Of(err) != exitcode.NotFound {
		t.Errorf("exit code = %d, want %d; err=%v", exitcode.Of(err), exitcode.NotFound, err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("message %q should say not found", err.Error())
	}
}

func TestStatObject_AccessDeniedIsExit5(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	srv.DenyAll = true
	sess := newRbStatSession(t, srv, "backups-7f3a")

	_, err := statObject(context.Background(), sess, "a.txt", "")
	if err == nil || exitcode.Of(err) != exitcode.Permission {
		t.Fatalf("expected exit 5, got %v (code %d)", err, exitcode.Of(err))
	}
	if strings.Contains(err.Error(), "SK") {
		t.Errorf("error must not include the secret: %q", err.Error())
	}
}

func TestStatUserMetadata(t *testing.T) {
	info := minio.ObjectInfo{
		UserMetadata: map[string]string{"Owner": "ops", "X-Amz-Meta-Env": "prod"},
		Metadata:     http.Header{"X-Amz-Meta-Team": {"storage"}, "Content-Type": {"text/plain"}, "X-Amz-Meta-Owner": {"ignored"}},
	}
	got := statUserMetadata(info)
	want := map[string]string{"owner": "ops", "env": "prod", "team": "storage"}
	if len(got) != len(want) {
		t.Fatalf("metadata = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("metadata[%q] = %q, want %q", k, got[k], v)
		}
	}
	if statUserMetadata(minio.ObjectInfo{}) != nil {
		t.Errorf("no metadata should yield nil")
	}
}

func TestWriteBucketStat(t *testing.T) {
	created := time.Date(2026, 9, 7, 15, 12, 1, 0, time.UTC)
	b := &objectstorage.Bucket{
		ID:            "bkt_1Gjbang9n0L2w",
		Name:          "backups",
		BucketName:    "backups-7f3a",
		Endpoint:      "https://s3.us-central-1.storage.sh",
		SigningRegion: "us-central-1",
		StorageClass:  "standard",
		Site:          "DAL",
		ProjectID:     "proj_1",
		ProjectSlug:   "my-project",
		Versioning:    true,
		Locking:       true,
		RetentionMode: "GOVERNANCE",
		RetentionDays: 30,
		Source:        "default",
		CreatedAt:     &created,
	}
	var out bytes.Buffer
	writeBucketStat(&out, b, []statCoveringKey{{Name: "lsh-me-standard", Permission: "fullaccess"}, {Name: "ci", Permission: "readonly"}})
	text := out.String()
	for _, want := range []string{
		"ID:", "bkt_1Gjbang9n0L2w",
		"Name:", "backups",
		"Bucket name (backend):", "backups-7f3a",
		"Project:", "my-project",
		"Class:", "standard",
		"Site:", "DAL",
		"Endpoint:", "https://s3.us-central-1.storage.sh",
		"Signing region:", "us-central-1",
		"Versioning:", "yes",
		"Locking:", "GOVERNANCE (30d)",
		"Source:", "default",
		"Created:",
		"Access keys covering this bucket:",
		"  lsh-me-standard (fullaccess)",
		"  ci (readonly)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	// Labels are aligned: every value starts in the same column.
	col := -1
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if !strings.Contains(line, ":") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "Access keys") {
			continue
		}
		i := strings.Index(line, ": ")
		rest := line[i+1:]
		valueCol := i + 1 + (len(rest) - len(strings.TrimLeft(rest, " ")))
		if col == -1 {
			col = valueCol
		} else if valueCol != col {
			t.Errorf("value column %d differs from %d in %q", valueCol, col, line)
		}
	}

	out.Reset()
	writeBucketStat(&out, b, nil)
	if !strings.Contains(out.String(), "(none saved)") {
		t.Errorf("empty key list should print (none saved):\n%s", out.String())
	}
}

func TestWriteBucketStat_EndpointOverride(t *testing.T) {
	b := &objectstorage.Bucket{BucketName: "raw-bucket", Endpoint: "https://objects.tyo4.storage.sh", SigningRegion: "tyo4", EndpointOverride: true}
	var out bytes.Buffer
	writeBucketStat(&out, b, nil)
	text := out.String()
	if strings.Contains(text, "ID:") || strings.Contains(text, "Access keys") {
		t.Errorf("API-only fields must be skipped in endpoint-override mode:\n%s", text)
	}
	if !strings.Contains(text, "raw-bucket") || !strings.Contains(text, "tyo4") {
		t.Errorf("backend name and signing region should be shown:\n%s", text)
	}
}

func TestStatCoveringKeys(t *testing.T) {
	b := &objectstorage.Bucket{ID: "bkt_1", StorageClass: "standard", Site: "DAL", ProjectID: "proj_1"}
	keys := map[string]config.StoredAccessKey{
		"full":        {Scope: config.ScopeFullAccess, StorageClass: "standard", ProjectID: "proj_1"},
		"limited-rw":  {Scope: config.ScopeLimitedAccess, StorageClass: "standard", Buckets: map[string]string{"bkt_1": "rw"}},
		"limited-ro":  {Scope: config.ScopeLimitedAccess, StorageClass: "standard", Buckets: map[string]string{"bkt_1": "readonly"}},
		"other-bkt":   {Scope: config.ScopeLimitedAccess, StorageClass: "standard", Buckets: map[string]string{"bkt_2": "rw"}},
		"other-class": {Scope: config.ScopeFullAccess, StorageClass: "high_performance", Site: "TYO4"},
		"other-proj":  {Scope: config.ScopeFullAccess, StorageClass: "standard", ProjectID: "proj_2"},
		"unknown":     {Scope: config.ScopeUnknown},
	}
	got := statCoveringKeys(keys, b)
	if len(got) != 3 {
		t.Fatalf("covering keys = %+v, want full, limited-ro, limited-rw", got)
	}
	if got[0].Name != "full" || got[0].Permission != "fullaccess" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "limited-ro" || got[1].Permission != "readonly" {
		t.Errorf("got[1] = %+v", got[1])
	}
	if got[2].Name != "limited-rw" || got[2].Permission != "rw" {
		t.Errorf("got[2] = %+v", got[2])
	}
}

// TestBucketRowStructuredWithoutAPIPayload covers endpoint-override mode: there
// is no API document to marshal, so the row used to render as "{}" while the
// human output showed the bucket name, endpoint and signing region.
func TestBucketRowStructuredWithoutAPIPayload(t *testing.T) {
	b := &objectstorage.Bucket{
		Name: "backups-7f3a", BucketName: "backups-7f3a",
		Endpoint: "https://s3.us-central-1.storage.sh", SigningRegion: "us-central-1",
		EndpointOverride: true,
	}
	raw, err := json.Marshal(NewBucketRow(b))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]string
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	for k, want := range map[string]string{
		"name": "backups-7f3a", "bucket_name": "backups-7f3a",
		"endpoint": "https://s3.us-central-1.storage.sh", "signing_region": "us-central-1",
	} {
		if doc[k] != want {
			t.Errorf("%s = %q, want %q (in %s)", k, doc[k], want, raw)
		}
	}
}
