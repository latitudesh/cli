package s3

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
)

// rmTestSession opens a Session against the fake S3 server.
func rmTestSession(t *testing.T, srv *s3test.Server, bucketName string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "b", BucketName: bucketName, Endpoint: srv.URL(), StorageClass: "standard", SigningRegion: "us-east-1"}
	cred := objectstorage.NewCredential("AK", "SK", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

func rmYes(string) error { return nil }

func rmFilters(t *testing.T, rules ...string) *objectstorage.Filters {
	t.Helper()
	f := &objectstorage.Filters{}
	for i := 0; i+1 < len(rules); i += 2 {
		if err := f.Add(rules[i] == "include", rules[i+1]); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestRmParseArgs(t *testing.T) {
	cases := []struct {
		name     string
		arg      string
		opts     rmOptions
		wantKey  string
		wantCode int
		wantMsg  string
	}{
		{name: "single object", arg: "s3://logs/tmp/old.log", wantKey: "tmp/old.log"},
		{name: "single without scheme", arg: "logs/tmp/old.log", wantKey: "tmp/old.log"},
		{name: "single trailing slash needs recursive", arg: "s3://logs/tmp/", wantCode: exitcode.Usage, wantMsg: "--recursive"},
		{name: "single bucket only", arg: "s3://logs", wantCode: exitcode.Usage, wantMsg: "names a bucket"},
		{name: "recursive normalizes prefix", arg: "s3://logs/tmp", opts: rmOptions{Recursive: true}, wantKey: "tmp/"},
		{name: "recursive keeps trailing slash", arg: "s3://logs/tmp/", opts: rmOptions{Recursive: true}, wantKey: "tmp/"},
		{name: "recursive whole bucket needs all", arg: "s3://logs", opts: rmOptions{Recursive: true}, wantCode: exitcode.Usage, wantMsg: "refusing to delete every object in s3://logs without --all"},
		{name: "recursive slash only needs all", arg: "s3://logs/", opts: rmOptions{Recursive: true}, wantCode: exitcode.Usage, wantMsg: "without --all"},
		{name: "recursive double slash needs all", arg: "s3://logs//", opts: rmOptions{Recursive: true}, wantCode: exitcode.Usage, wantMsg: "without --all"},
		{name: "recursive whole bucket with all", arg: "s3://logs", opts: rmOptions{Recursive: true, All: true}, wantKey: ""},
		{name: "all without recursive", arg: "s3://logs/k", opts: rmOptions{All: true}, wantCode: exitcode.Usage, wantMsg: "--all"},
		{name: "versions without recursive", arg: "s3://logs/k", opts: rmOptions{Versions: true}, wantCode: exitcode.Usage, wantMsg: "--versions"},
		{name: "version-id with recursive", arg: "s3://logs/k/", opts: rmOptions{Recursive: true, VersionID: "v1"}, wantCode: exitcode.Usage, wantMsg: "--version-id"},
		{name: "negative max-delete", arg: "s3://logs/k", opts: rmOptions{MaxDelete: -1}, wantCode: exitcode.Usage, wantMsg: "--max-delete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := rmParseArgs(tc.arg, tc.opts)
			if tc.wantCode != 0 {
				if err == nil {
					t.Fatalf("expected error, got ref %+v", ref)
				}
				if exitcode.Of(err) != tc.wantCode {
					t.Errorf("exit code = %d, want %d (%v)", exitcode.Of(err), tc.wantCode, err)
				}
				if !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("error %q does not mention %q", err, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref.Key != tc.wantKey {
				t.Errorf("key = %q, want %q", ref.Key, tc.wantKey)
			}
		})
	}
}

func TestRmSingleObjectNoHeadAndIdempotent(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/old.log", []byte("x"), "text/plain")
	srv.AddObject("logs-7f3a", "tmp/keep.log", []byte("y"), "text/plain")
	sess := rmTestSession(t, srv, "logs-7f3a")

	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/old.log", Remote: true}, rmOptions{}, true, &out, rmYes)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "delete: s3://b/tmp/old.log\n" {
		t.Errorf("stdout = %q", got)
	}
	if len(rows) != 1 || !rows[0].Deleted || rows[0].Key != "tmp/old.log" {
		t.Errorf("rows = %+v", rows)
	}
	if srv.Object("logs-7f3a", "tmp/old.log") != nil {
		t.Error("object still present")
	}
	if srv.Object("logs-7f3a", "tmp/keep.log") == nil {
		t.Error("sibling object was deleted")
	}
	for _, r := range srv.Requests() {
		if r.Method == http.MethodHead || r.Method == http.MethodGet {
			t.Errorf("unexpected %s %s: single delete must not HEAD or list", r.Method, r.Path)
		}
		if r.Method == http.MethodDelete && r.Path != "/logs-7f3a/tmp/old.log" {
			t.Errorf("unexpected delete path %s", r.Path)
		}
	}

	// Missing key: still exit 0 and still reported as deleted.
	srv.ResetRequests()
	out.Reset()
	rows, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/missing.log", Remote: true}, rmOptions{}, true, &out, rmYes)
	if err != nil {
		t.Fatalf("delete of a missing key must succeed: %v", err)
	}
	if len(rows) != 1 || out.String() != "delete: s3://b/tmp/missing.log\n" {
		t.Errorf("rows=%+v out=%q", rows, out.String())
	}
}

func TestRmSingleVersionIDAndQuiet(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("vers-7f3a").Versioned = true
	// Each put on a versioned bucket gets the next version id (v1, v2…).
	srv.AddObject("vers-7f3a", "k", []byte("1"), "")
	current := srv.AddObject("vers-7f3a", "k", []byte("22"), "")
	if current.VersionID != "v2" {
		t.Fatalf("fake server version id = %q, want v2", current.VersionID)
	}
	sess := rmTestSession(t, srv, "vers-7f3a")

	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "k", Remote: true}, rmOptions{VersionID: "v2", Quiet: true}, true, &out, rmYes)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("--quiet printed %q", out.String())
	}
	if len(rows) != 1 || rows[0].VersionID != "v2" {
		t.Errorf("rows = %+v", rows)
	}
	var sawVersion bool
	for _, r := range srv.WriteRequests() {
		if r.Method == http.MethodDelete && r.Query.Get("versionId") == "v2" {
			sawVersion = true
		}
	}
	if !sawVersion {
		t.Error("DELETE did not carry versionId=v2")
	}
	if o := srv.Object("vers-7f3a", "k"); o == nil || string(o.Data) != "1" || o.VersionID != "v1" {
		t.Errorf("previous version (v1) should be current now, got %+v", o)
	}
}

func TestRmRecursiveWithFiltersAndCount(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/a.log", []byte("aaa"), "")
	srv.AddObject("logs-7f3a", "tmp/b.log", []byte("bb"), "")
	srv.AddObject("logs-7f3a", "tmp/notes.txt", []byte("n"), "")
	srv.AddObject("logs-7f3a", "tmp/sub/c.log", []byte("c"), "")
	srv.AddObject("logs-7f3a", "tmp2/d.log", []byte("d"), "")
	srv.AddObject("logs-7f3a", "other.log", []byte("o"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	var out bytes.Buffer
	var asked string
	confirm := func(q string) error { asked = q; return nil }
	opts := rmOptions{Recursive: true, Filters: rmFilters(t, "exclude", "*", "include", "*.log")}
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, opts, true, &out, confirm)
	if err != nil {
		t.Fatal(err)
	}
	if asked != "Delete 3 objects under s3://b/tmp/?" {
		t.Errorf("confirmation = %q", asked)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	want := "delete: s3://b/tmp/a.log\ndelete: s3://b/tmp/b.log\ndelete: s3://b/tmp/sub/c.log\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
	if keys := srv.Keys("logs-7f3a"); strings.Join(keys, ",") != "other.log,tmp/notes.txt,tmp2/d.log" {
		t.Errorf("remaining keys = %v", keys)
	}
	// One multi-delete POST for the batch, no per-object DELETEs.
	var posts, deletes int
	for _, r := range srv.WriteRequests() {
		switch r.Method {
		case http.MethodPost:
			posts++
			if !r.Query.Has("delete") {
				t.Errorf("POST without ?delete: %s", r.Path)
			}
		case http.MethodDelete:
			deletes++
		}
	}
	if posts != 1 || deletes != 0 {
		t.Errorf("posts=%d deletes=%d, want one multi-delete", posts, deletes)
	}
}

func TestRmRecursiveStructuredRowsAndOnlyShowErrors(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/a.log", []byte("a"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, OnlyShowErrors: true}, true, &out, rmYes)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("--only-show-errors printed %q", out.String())
	}
	if len(rows) != 1 || !rows[0].Deleted || rows[0].Bucket != "b" || rows[0].Key != "tmp/a.log" {
		t.Errorf("rows = %+v", rows)
	}
	// Structured mode never writes to out either.
	srv.AddObject("logs-7f3a", "tmp/b.log", []byte("b"), "")
	out.Reset()
	rows, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true}, false, &out, rmYes)
	if err != nil || out.Len() != 0 || len(rows) != 1 {
		t.Errorf("structured: err=%v out=%q rows=%+v", err, out.String(), rows)
	}
}

func TestRmRecursiveEmptyPrefixIsNoop(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "keep.log", []byte("k"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	called := false
	confirm := func(string) error { called = true; return nil }
	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "nothing/", Remote: true}, rmOptions{Recursive: true}, true, &out, confirm)
	if err != nil || len(rows) != 0 || out.Len() != 0 {
		t.Errorf("err=%v rows=%+v out=%q", err, rows, out.String())
	}
	if called {
		t.Error("no confirmation should be asked when nothing matches")
	}
	if len(srv.WriteRequests()) != 0 {
		t.Error("no writes expected")
	}
}

func TestRmRecursiveMaxDeleteRefusesWithoutWrites(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	for _, k := range []string{"tmp/a", "tmp/b", "tmp/c"} {
		srv.AddObject("logs-7f3a", k, []byte("x"), "")
	}
	sess := rmTestSession(t, srv, "logs-7f3a")

	called := false
	confirm := func(string) error { called = true; return nil }
	var out bytes.Buffer
	_, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, MaxDelete: 2}, true, &out, confirm)
	if err == nil || exitcode.Of(err) != exitcode.Refused {
		t.Fatalf("expected exit %d, got %v", exitcode.Refused, err)
	}
	if !strings.Contains(err.Error(), "3 objects match") || !strings.Contains(err.Error(), "--max-delete is 2") {
		t.Errorf("message = %q", err)
	}
	if called {
		t.Error("must refuse before asking for confirmation")
	}
	if n := len(srv.WriteRequests()); n != 0 {
		t.Errorf("%d write requests, want 0", n)
	}
	if len(srv.Keys("logs-7f3a")) != 3 {
		t.Error("objects were deleted")
	}

	// Exactly at the limit is allowed.
	_, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, MaxDelete: 3}, true, &out, rmYes)
	if err != nil {
		t.Fatalf("max-delete equal to the count must pass: %v", err)
	}
	if len(srv.Keys("logs-7f3a")) != 0 {
		t.Error("objects not deleted")
	}
}

func TestRmRecursiveRefusedConfirmation(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/a", []byte("x"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	refuse := func(q string) error { return exitcode.Errorf(exitcode.Refused, "cancelled") }
	var out bytes.Buffer
	_, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true}, true, &out, refuse)
	if exitcode.Of(err) != exitcode.Refused {
		t.Fatalf("expected exit 7, got %v", err)
	}
	if len(srv.WriteRequests()) != 0 || srv.Object("logs-7f3a", "tmp/a") == nil {
		t.Error("declined confirmation must not delete")
	}
}

func TestRmDryRunZeroWrites(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/a.log", []byte("aaaa"), "")
	srv.AddObject("logs-7f3a", "tmp/b.txt", []byte("bb"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	called := false
	confirm := func(string) error { called = true; return nil }
	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, DryRun: true}, true, &out, confirm)
	if err != nil {
		t.Fatal(err)
	}
	want := "(dryrun) delete: s3://b/tmp/a.log\n(dryrun) delete: s3://b/tmp/b.txt\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
	if len(rows) != 2 || !rows[0].DryRun || rows[0].Deleted {
		t.Errorf("rows = %+v", rows)
	}
	if called {
		t.Error("dry-run must not prompt")
	}
	if n := len(srv.WriteRequests()); n != 0 {
		t.Errorf("%d write requests during dry-run", n)
	}
	if len(srv.Keys("logs-7f3a")) != 2 {
		t.Error("dry-run deleted objects")
	}

	// Single-object dry-run: no request at all.
	srv.ResetRequests()
	out.Reset()
	rows, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/a.log", Remote: true}, rmOptions{DryRun: true}, true, &out, confirm)
	if err != nil || len(rows) != 1 || out.String() != "(dryrun) delete: s3://b/tmp/a.log\n" {
		t.Errorf("single dry-run: err=%v rows=%+v out=%q", err, rows, out.String())
	}
	if len(srv.Requests()) != 0 {
		t.Error("single-object dry-run must not call the backend")
	}
}

func TestRmRecursiveVersions(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("vers-7f3a").Versioned = true
	srv.AddObject("vers-7f3a", "tmp/a", []byte("1"), "")
	srv.AddObject("vers-7f3a", "tmp/a", []byte("22"), "")
	srv.AddObject("vers-7f3a", "tmp/b", []byte("3"), "")
	sess := rmTestSession(t, srv, "vers-7f3a")

	// Without --versions only the current objects are targeted.
	var out bytes.Buffer
	var asked string
	confirm := func(q string) error { asked = q; return nil }
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, DryRun: true}, true, &out, confirm)
	if err != nil || len(rows) != 2 {
		t.Fatalf("plain plan: err=%v rows=%+v", err, rows)
	}

	// With --versions every version is enumerated and deleted.
	out.Reset()
	rows, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, Versions: true}, true, &out, confirm)
	if err != nil {
		t.Fatal(err)
	}
	if asked != "Delete 3 object versions under s3://b/tmp/?" {
		t.Errorf("confirmation = %q", asked)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	perKey := map[string]int{}
	for _, r := range rows {
		if !r.Deleted || r.VersionID == "" {
			t.Errorf("row %+v should be a deleted version", r)
		}
		perKey[r.Key]++
	}
	if perKey["tmp/a"] != 2 || perKey["tmp/b"] != 1 {
		t.Errorf("versions per key = %v, want tmp/a:2 tmp/b:1", perKey)
	}
	if !strings.Contains(out.String(), "delete: s3://b/tmp/a (version v1)") {
		t.Errorf("stdout = %q", out.String())
	}
	if len(srv.Keys("vers-7f3a")) != 0 {
		t.Errorf("remaining keys %v", srv.Keys("vers-7f3a"))
	}
	var listedVersions bool
	for _, r := range srv.Requests() {
		if r.Method == http.MethodGet && r.Query.Has("versions") {
			listedVersions = true
		}
	}
	if !listedVersions {
		t.Error("expected a ListObjectVersions call")
	}
}

func TestRmWholeBucketWithAll(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "a", []byte("1"), "")
	srv.AddObject("logs-7f3a", "dir/b", []byte("2"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")
	sess.Bucket.ProjectSlug = "my-project"

	var asked string
	confirm := func(q string) error { asked = q; return nil }
	var out bytes.Buffer
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "", Remote: true}, rmOptions{Recursive: true, All: true}, true, &out, confirm)
	if err != nil || len(rows) != 2 {
		t.Fatalf("err=%v rows=%+v", err, rows)
	}
	for _, want := range []string{"Delete all 2 objects in s3://b", "bkt_1", "backend name logs-7f3a", "project my-project"} {
		if !strings.Contains(asked, want) {
			t.Errorf("confirmation %q lacks %q", asked, want)
		}
	}
	if len(srv.Keys("logs-7f3a")) != 0 {
		t.Error("bucket not emptied")
	}
}

func TestRmObjectLockRefusals(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("lock-7f3a")
	srv.AddObject("lock-7f3a", "tmp/a", []byte("1"), "")
	sess := rmTestSession(t, srv, "lock-7f3a")
	sess.Bucket.Locking = true
	sess.Bucket.RetentionMode = "COMPLIANCE"

	var out bytes.Buffer
	_, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true}, true, &out, rmYes)
	if exitcode.Of(err) != exitcode.Refused || !strings.Contains(err.Error(), "COMPLIANCE") {
		t.Errorf("compliance: %v", err)
	}
	sess.Bucket.RetentionMode = "GOVERNANCE"
	_, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true}, true, &out, rmYes)
	if exitcode.Of(err) != exitcode.Refused || !strings.Contains(err.Error(), "--bypass-governance-retention") {
		t.Errorf("governance: %v", err)
	}
	if len(srv.Requests()) != 0 {
		t.Error("lock refusals must happen before any request")
	}
	// Bypass: proceeds and sends the bypass header.
	_, err = runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true, BypassGovernance: true}, true, &out, rmYes)
	if err != nil {
		t.Fatal(err)
	}
	var sawBypass bool
	for _, r := range srv.WriteRequests() {
		if strings.EqualFold(r.Header.Get("X-Amz-Bypass-Governance-Retention"), "true") {
			sawBypass = true
		}
	}
	if !sawBypass {
		t.Error("multi-delete did not carry x-amz-bypass-governance-retention")
	}
}

func TestRmRecursivePartialFailure(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	srv.AddObject("logs-7f3a", "tmp/a", []byte("1"), "")
	sess := rmTestSession(t, srv, "logs-7f3a")

	var out bytes.Buffer
	// Listing succeeds; the multi-delete POST is denied.
	confirm := func(string) error {
		srv.FailNext = &s3test.ErrorResponse{Code: "AccessDenied", Message: "Access Denied", Status: 403}
		return nil
	}
	rows, err := runRm(context.Background(), sess, objectstorage.Ref{Bucket: "b", Key: "tmp/", Remote: true}, rmOptions{Recursive: true}, true, &out, confirm)
	if exitcode.Of(err) != exitcode.Partial {
		t.Fatalf("expected exit %d, got %v", exitcode.Partial, err)
	}
	if !strings.Contains(err.Error(), "0 deleted, 1 failed") {
		t.Errorf("summary = %q", err)
	}
	if len(rows) != 1 || rows[0].Deleted || !strings.Contains(rows[0].Error, "AccessDenied") {
		t.Errorf("rows = %+v", rows)
	}
	if out.Len() != 0 {
		t.Errorf("failures must not go to stdout: %q", out.String())
	}
}

// rmCaptureOutput runs fn with os.Stdout and os.Stderr redirected to pipes and
// returns what was written to each. Command-level tests need it because the
// rm/presign RunE and objectstorage.PrintError write straight to the process
// streams (and cobra echoes errors to os.Stderr unless silenced).
func rmCaptureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	redirect := func(target **os.File) func() string {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		old := *target
		*target = w
		done := make(chan string, 1)
		go func() {
			b, _ := io.ReadAll(r)
			done <- string(b)
		}()
		return func() string {
			_ = w.Close()
			*target = old
			return <-done
		}
	}
	finishOut := redirect(&os.Stdout)
	finishErr := redirect(&os.Stderr)
	fn()
	// Restore stderr first so a failing assertion below is visible.
	stderr = finishErr()
	stdout = finishOut()
	return stdout, stderr
}

// TestRmCommandRecursiveWithoutAllIsUsageError drives the cobra command end to
// end: `rm s3://bucket --recursive` without --all must exit 2 through the
// newCmd wrapper and print the refusal exactly once (RunE prints it, cobra is
// silenced). It fails before any bucket lookup, so no API or S3 call is made.
func TestRmCommandRecursiveWithoutAllIsUsageError(t *testing.T) {
	for _, target := range []string{"s3://logs", "s3://logs/"} {
		t.Run(target, func(t *testing.T) {
			cmd := NewRmCmd()
			Finalize(cmd) // production installs this in build_s3.go
			cmd.SetArgs([]string{target, "--recursive"})
			var err error
			stdout, stderr := rmCaptureOutput(t, func() { err = cmd.Execute() })
			if exitcode.Of(err) != exitcode.Usage {
				t.Fatalf("exit code = %d, want %d (%v)", exitcode.Of(err), exitcode.Usage, err)
			}
			if n := strings.Count(stderr, "refusing to delete every object in s3://logs without --all"); n != 1 {
				t.Errorf("refusal printed %d times, want exactly once:\n%s", n, stderr)
			}
			if strings.Contains(stderr, "Usage:") {
				t.Errorf("usage must not be echoed on errors:\n%s", stderr)
			}
			if stdout != "" {
				t.Errorf("nothing should reach stdout, got %q", stdout)
			}
		})
	}
}

// TestRmCommandSingleMissingKeyExitsZero runs a plain `rm` of a key that does
// not exist through the s3 group in --endpoint-url mode against the fake S3
// server: S3 deletes are idempotent, so the command exits 0 and reports the
// key as deleted.
func TestRmCommandSingleMissingKeyExitsZero(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("logs-7f3a")
	t.Setenv(objectstorage.EnvAccessKeyID, "AK")
	t.Setenv(objectstorage.EnvSecretAccessKey, "SK")

	group := NewGroupCmd()
	group.AddCommand(NewRmCmd())
	Finalize(group) // production installs this in build_s3.go
	group.SetArgs([]string{"rm", "s3://logs-7f3a/tmp/missing.log", "--endpoint-url", srv.URL()})
	var err error
	stdout, stderr := rmCaptureOutput(t, func() { err = group.Execute() })
	if err != nil {
		t.Fatalf("rm of a missing key must exit 0, got %v\nstderr: %s", err, stderr)
	}
	if stdout != "delete: s3://logs-7f3a/tmp/missing.log\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if strings.Contains(stderr, "Error") {
		t.Errorf("no error expected on stderr:\n%s", stderr)
	}
	var deletes int
	for _, r := range srv.Requests() {
		if r.Method == http.MethodDelete {
			deletes++
		}
		if r.Method == http.MethodHead || r.Method == http.MethodGet {
			t.Errorf("unexpected %s %s: single delete must not HEAD or list", r.Method, r.Path)
		}
	}
	if deletes != 1 {
		t.Errorf("%d DELETE requests, want 1", deletes)
	}
}

// TestRmCommandRejectsRegionFlag guards the aws-compat shim: --region is an
// unsupported flag on object commands and must fail with exit 2 and the
// pointer to --signing-region, before any network call.
func TestRmCommandRejectsRegionFlag(t *testing.T) {
	cmd := NewRmCmd()
	Finalize(cmd)
	cmd.SetArgs([]string{"s3://logs/tmp/old.log", "--region", "us-east-1"})
	var err error
	_, stderr := rmCaptureOutput(t, func() { err = cmd.Execute() })
	if exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d (%v)", exitcode.Of(err), exitcode.Usage, err)
	}
	if !strings.Contains(stderr, "--region is not supported") || !strings.Contains(stderr, "--signing-region") {
		t.Errorf("stderr lacks the directed --region explanation:\n%s", stderr)
	}
}

// TestRmBucketOnlyPointsAtRb covers the migration path of the legacy
// `storage-objects rm <bkt_id>`, which deleted a bucket: that alias now lands
// on `s3 rm`, so the usage error names both destinations.
func TestRmBucketOnlyPointsAtRb(t *testing.T) {
	_, err := rmParseArgs("bkt_abc123", rmOptions{})
	if err == nil {
		t.Fatal("expected a usage error for a bucket-only target")
	}
	if code := exitcode.Of(err); code != exitcode.Usage {
		t.Errorf("exit code = %d, want %d", code, exitcode.Usage)
	}
	for _, want := range []string{"names a bucket, not an object", "--recursive --all", "lsh s3 rb s3://bkt_abc123"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
	// A prefix keeps the shorter --recursive hint.
	if _, err := rmParseArgs("s3://logs/tmp/", rmOptions{}); err == nil || strings.Contains(err.Error(), "lsh s3 rb") {
		t.Errorf("a prefix must keep the --recursive hint, got %v", err)
	}
}
