package objectstorage

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
	"github.com/minio/minio-go/v7"
)

// TestClientAgainstFakeS3 exercises the client settings that matter for
// S3-compatible backends: path-style addressing, the derived signing region,
// no checksum trailers on writes, and that the fake server round-trips
// list/put/get/delete.
func TestClientAgainstFakeS3(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	srv.AddObject("backups-7f3a", "2026/09/a.sql", []byte("aaa"), "application/sql")
	srv.AddObject("backups-7f3a", "2026/09/b.sql", []byte("bbbb"), "application/sql")
	srv.AddObject("backups-7f3a", "readme.txt", []byte("hi"), "text/plain")

	b := &Bucket{ID: "bkt_1", Name: "backups", BucketName: "backups-7f3a", Endpoint: srv.URL(), StorageClass: "standard", SigningRegion: "eu-west-2"}
	var trace bytes.Buffer
	client, err := NewS3Client(b, NewCredential("AKIAEXAMPLE", "topsecret", "test"), ClientOptions{Debug: true, Trace: &trace, MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Non-recursive listing: one prefix + one object.
	var keys []string
	var prefixes []string
	for info := range client.ListObjects(ctx, b.BucketName, minio.ListObjectsOptions{Recursive: false}) {
		if info.Err != nil {
			t.Fatal(info.Err)
		}
		if strings.HasSuffix(info.Key, "/") {
			prefixes = append(prefixes, info.Key)
		} else {
			keys = append(keys, info.Key)
		}
	}
	if len(prefixes) != 1 || prefixes[0] != "2026/" || len(keys) != 1 || keys[0] != "readme.txt" {
		t.Fatalf("listing: prefixes=%v keys=%v", prefixes, keys)
	}

	// Upload, download, delete.
	if _, err := client.PutObject(ctx, b.BucketName, "new/file.bin", bytes.NewReader([]byte("payload")), 7, minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
		t.Fatal(err)
	}
	obj, err := client.GetObject(ctx, b.BucketName, "new/file.bin", minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(obj)
	if err != nil || string(data) != "payload" {
		t.Fatalf("download = %q, %v", data, err)
	}
	if err := client.RemoveObject(ctx, b.BucketName, "new/file.bin", minio.RemoveObjectOptions{}); err != nil {
		t.Fatal(err)
	}
	if srv.Object("backups-7f3a", "new/file.bin") != nil {
		t.Fatal("object still present after delete")
	}
	// Deleting a missing key is idempotent (S3 semantics).
	if err := client.RemoveObject(ctx, b.BucketName, "missing", minio.RemoveObjectOptions{}); err != nil {
		t.Fatalf("delete of missing key must succeed: %v", err)
	}

	reqs := srv.Requests()
	if len(reqs) == 0 {
		t.Fatal("no requests recorded")
	}
	for _, r := range reqs {
		if !strings.HasPrefix(r.Path, "/backups-7f3a") {
			t.Errorf("expected path-style request, got %s", r.Path)
		}
		if region := s3test.SigningRegionOf(r); region != "eu-west-2" {
			t.Errorf("signing region = %q, want eu-west-2 (%s %s)", region, r.Method, r.Path)
		}
	}
	if srv.HasChecksumHeaders() {
		t.Error("writes must not carry x-amz-checksum-*/aws-chunked headers (Wasabi rejects them)")
	}
	if strings.Contains(trace.String(), "topsecret") {
		t.Error("trace leaked the secret")
	}
	// minio already masks the SigV4 signature in its trace; our writer must
	// keep it that way (no 64-hex signature anywhere in the output).
	if regexp.MustCompile(`Signature=[0-9a-f]{64}`).MatchString(trace.String()) {
		t.Error("trace did not redact the signature")
	}
}

func TestHumanizeMapsS3Errors(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bkt-test")
	b := &Bucket{ID: "bkt_1", Name: "b", BucketName: "bkt-test", Endpoint: srv.URL(), SigningRegion: "us-east-1"}
	cred := NewCredential("AK", "SK", "saved key \"x\"")
	client, err := NewS3Client(b, cred, ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_, err = client.StatObject(ctx, "bkt-test", "nope", minio.StatObjectOptions{})
	if h := Humanize(err, b, &cred); h == nil || !strings.Contains(h.Error(), "not found") {
		t.Errorf("NoSuchKey → %v", h)
	}

	srv.DenyAll = true
	_, err = client.StatObject(ctx, "bkt-test", "x", minio.StatObjectOptions{})
	h := Humanize(err, b, &cred)
	if h == nil || !strings.Contains(h.Error(), "access denied") {
		t.Errorf("AccessDenied → %v", h)
	}
	srv.DenyAll = false

	// HEAD responses carry no body, so use a GET (listing) to exercise the
	// XML error mapping.
	srv.FailNext = &s3test.ErrorResponse{Code: "AuthorizationHeaderMalformed", Message: "wrong region", Region: "eu-west-2", Status: 400}
	err = nil
	for info := range client.ListObjects(ctx, "bkt-test", minio.ListObjectsOptions{}) {
		if info.Err != nil {
			err = info.Err
		}
	}
	h = Humanize(err, b, &cred)
	if h == nil || !strings.Contains(h.Error(), "eu-west-2") {
		t.Errorf("AuthorizationHeaderMalformed → %v", h)
	}
}
