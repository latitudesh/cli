package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
)

// newRbStatSession builds a Session against the fake S3 server for the rb and
// stat tests.
func newRbStatSession(t *testing.T, srv *s3test.Server, backendName string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{
		ID:            "bkt_1",
		Name:          "backups",
		BucketName:    backendName,
		Endpoint:      srv.URL(),
		StorageClass:  "standard",
		SigningRegion: "us-east-1",
		ProjectSlug:   "my-project",
	}
	cred := objectstorage.NewCredential("AK", "SK", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

// stubRbDeleteAPI replaces the API delete for the test and reports calls.
func stubRbDeleteAPI(t *testing.T, fail error) *int {
	t.Helper()
	calls := 0
	prev := rbDeleteBucketAPI
	rbDeleteBucketAPI = func(ctx context.Context, b *objectstorage.Bucket) error {
		calls++
		return fail
	}
	t.Cleanup(func() { rbDeleteBucketAPI = prev })
	return &calls
}

func rbSeedBucket(srv *s3test.Server, name string, n int) {
	srv.CreateBucket(name)
	for i := 0; i < n; i++ {
		srv.AddObject(name, fmt.Sprintf("dir/file-%02d.txt", i), []byte("payload"), "text/plain")
	}
}

func TestForceRemoveBucket_DeletesObjectsThenBucket(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 3)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	calls := stubRbDeleteAPI(t, nil)

	var out bytes.Buffer
	rows, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Human: true}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("API delete called %d times, want 1", *calls)
	}
	if keys := srv.Keys("backups-7f3a"); len(keys) != 0 {
		t.Errorf("objects left on the backend: %v", keys)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 3 delete lines + remove_bucket, got %d:\n%s", len(lines), out.String())
	}
	for _, l := range lines[:3] {
		if !strings.HasPrefix(l, "delete: s3://backups/dir/file-") {
			t.Errorf("unexpected line %q", l)
		}
	}
	if lines[3] != "remove_bucket: s3://backups" {
		t.Errorf("last line = %q", lines[3])
	}
	if len(rows) != 4 || !rows[3].Deleted || rows[3].Key != "" {
		t.Errorf("rows = %+v", rows)
	}
	// Deletes go through multi-object POSTs, never one DELETE per object.
	for _, r := range srv.WriteRequests() {
		if r.Method == "DELETE" {
			t.Errorf("unexpected single DELETE request %s", r.Path)
		}
	}
}

func TestForceRemoveBucket_MaxDeleteRefuses(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 5)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	calls := stubRbDeleteAPI(t, nil)

	var out bytes.Buffer
	_, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Human: true, MaxDelete: 2}, &out)
	if err == nil {
		t.Fatalf("expected a refusal")
	}
	if exitcode.Of(err) != exitcode.Refused {
		t.Errorf("exit code = %d, want %d", exitcode.Of(err), exitcode.Refused)
	}
	if !strings.Contains(err.Error(), "--max-delete 2") || !strings.Contains(err.Error(), "5 objects") {
		t.Errorf("message %q should mention the limit and the count", err.Error())
	}
	if *calls != 0 {
		t.Errorf("API delete must not be called, got %d", *calls)
	}
	if len(srv.WriteRequests()) != 0 {
		t.Errorf("no write must reach the backend, got %d", len(srv.WriteRequests()))
	}
	if len(srv.Keys("backups-7f3a")) != 5 {
		t.Errorf("objects were deleted despite the refusal")
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be printed on stdout, got %q", out.String())
	}
}

func TestForceRemoveBucket_DryRunWritesNothing(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 2)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	calls := stubRbDeleteAPI(t, nil)

	var out bytes.Buffer
	// Yes is deliberately false: dry-run never prompts.
	rows, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{DryRun: true, Human: true}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 0 {
		t.Errorf("API delete must not be called in dry-run, got %d", *calls)
	}
	if len(srv.WriteRequests()) != 0 {
		t.Errorf("dry-run must not write, got %d write requests", len(srv.WriteRequests()))
	}
	if len(srv.Keys("backups-7f3a")) != 2 {
		t.Errorf("dry-run deleted objects")
	}
	want := "(dryrun) delete: s3://backups/dir/file-00.txt\n(dryrun) delete: s3://backups/dir/file-01.txt\n(dryrun) remove_bucket: s3://backups\n"
	if out.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	if len(rows) != 3 || !rows[0].DryRun || !rows[2].DryRun {
		t.Errorf("rows = %+v", rows)
	}
}

func TestForceRemoveBucket_VersionedNeedsVersionsFlag(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	bkt := srv.CreateBucket("backups-7f3a")
	bkt.Versioned = true
	// The fake server derives version IDs from the history length before
	// appending, so pin distinct IDs explicitly.
	srv.AddObject("backups-7f3a", "a.txt", []byte("v1"), "text/plain").VersionID = "v1"
	srv.AddObject("backups-7f3a", "a.txt", []byte("v2"), "text/plain").VersionID = "v2"
	sess := newRbStatSession(t, srv, "backups-7f3a")
	sess.Bucket.Versioning = true
	calls := stubRbDeleteAPI(t, nil)

	var out bytes.Buffer
	_, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Human: true}, &out)
	if err == nil || exitcode.Of(err) != exitcode.Refused {
		t.Fatalf("expected exit 7 refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "--versions") || !strings.Contains(err.Error(), "delete markers") {
		t.Errorf("message %q should point to --versions", err.Error())
	}
	if *calls != 0 || len(srv.Requests()) != 0 {
		t.Errorf("refusal must happen before any request (api=%d, s3=%d)", *calls, len(srv.Requests()))
	}

	// With --versions every version goes away.
	out.Reset()
	rows, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Human: true, Versions: true}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("API delete called %d times, want 1", *calls)
	}
	if len(srv.Keys("backups-7f3a")) != 0 {
		t.Errorf("versions left: %v", srv.Keys("backups-7f3a"))
	}
	if len(rows) != 3 { // v1, v2 and the bucket
		t.Errorf("rows = %+v", rows)
	}
	if !strings.Contains(out.String(), "(version v1)") || !strings.Contains(out.String(), "(version v2)") {
		t.Errorf("output should list versions:\n%s", out.String())
	}
}

func TestForceRemoveBucket_ComplianceLockRefused(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 1)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	sess.Bucket.Locking = true
	sess.Bucket.Versioning = true
	sess.Bucket.RetentionMode = "COMPLIANCE"
	stubRbDeleteAPI(t, nil)

	_, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Versions: true, Human: true}, &bytes.Buffer{})
	if err == nil || exitcode.Of(err) != exitcode.Refused || !strings.Contains(err.Error(), "COMPLIANCE") {
		t.Fatalf("expected COMPLIANCE refusal, got %v", err)
	}
}

func TestForceRemoveBucket_NonInteractiveWithoutYesRefuses(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 1)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	calls := stubRbDeleteAPI(t, nil)

	_, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Human: true}, &bytes.Buffer{})
	if err == nil || exitcode.Of(err) != exitcode.Refused {
		t.Fatalf("expected exit 7 without --yes in a non-TTY, got %v", err)
	}
	for _, want := range []string{"backups", "bkt_1", "my-project", "backups-7f3a", "1 object"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("question %q should include %q", err.Error(), want)
		}
	}
	if *calls != 0 || len(srv.WriteRequests()) != 0 {
		t.Errorf("nothing must be deleted before confirmation")
	}
}

func TestForceRemoveBucket_APIDeleteFailureKeepsExitCode(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 1)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	stubRbDeleteAPI(t, exitcode.Errorf(exitcode.Permission, "your API token does not have permission"))

	var out bytes.Buffer
	rows, err := forceRemoveBucket(context.Background(), nil, sess, rbOptions{Yes: true, Human: true}, &out)
	if err == nil || exitcode.Of(err) != exitcode.Permission {
		t.Fatalf("expected the API error to propagate, got %v", err)
	}
	if len(rows) != 1 || !rows[0].Deleted {
		t.Errorf("object rows should still be returned: %+v", rows)
	}
	if strings.Contains(out.String(), "remove_bucket") {
		t.Errorf("remove_bucket must not be printed when the API delete failed")
	}
}

func TestRbDeleteObjects_BackendFailureIsPartial(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	rbSeedBucket(srv, "backups-7f3a", 2)
	sess := newRbStatSession(t, srv, "backups-7f3a")
	objects, err := rbListAllObjects(context.Background(), sess.Client, "backups-7f3a", false)
	if err != nil {
		t.Fatal(err)
	}
	srv.FailNext = &s3test.ErrorResponse{Code: "AccessDenied", Message: "Access Denied", Status: 403}

	var out bytes.Buffer
	rows, deleted, failed := rbDeleteObjects(context.Background(), sess, objects, rbOptions{Human: true}, &out)
	if deleted != 0 || failed == 0 {
		t.Fatalf("deleted=%d failed=%d, want a failed batch", deleted, failed)
	}
	if len(rows) == 0 || rows[0].Error == "" {
		t.Errorf("rows should carry the error: %+v", rows)
	}
	if !strings.Contains(out.String(), "delete failed:") {
		t.Errorf("human output should report the failure:\n%s", out.String())
	}
}

func TestRbHumanizeAPIDelete(t *testing.T) {
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "backups"}
	err := rbHumanizeAPIDelete(exitcode.Errorf(exitcode.Usage, "the API rejected the request: bucket is not empty"), b)
	if exitcode.Of(err) != exitcode.Refused || !strings.Contains(err.Error(), "--force") {
		t.Errorf("not-empty should become exit 7 with a --force hint, got %v", err)
	}
	other := exitcode.Errorf(exitcode.NotFound, "bucket not found")
	if got := rbHumanizeAPIDelete(other, b); !errors.Is(got, other) {
		t.Errorf("unrelated errors must pass through, got %v", got)
	}
	if rbHumanizeAPIDelete(nil, b) != nil {
		t.Errorf("nil must stay nil")
	}
}

func TestRbCommaInt(t *testing.T) {
	cases := map[int64]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 1204: "1,204", 1234567: "1,234,567", -1204: "-1,204"}
	for in, want := range cases {
		if got := rbCommaInt(in); got != want {
			t.Errorf("rbCommaInt(%d) = %q, want %q", in, got, want)
		}
	}
}
