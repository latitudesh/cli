package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
	"github.com/latitudesh/lsh/internal/renderer"
)

const lsTestBucket = "backups-7f3a"

// newLsTestSession builds a Session against the fake server.
func newLsTestSession(t *testing.T, srv *s3test.Server, bucket string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{
		ID:            "bkt_1",
		Name:          "backups",
		BucketName:    bucket,
		Endpoint:      srv.URL(),
		StorageClass:  "standard",
		SigningRegion: "us-east-1",
	}
	cred := objectstorage.NewCredential("AK", "SK", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

// seedLsBucket fills the bucket with objects under two prefixes plus one at
// the root and forces two-key pages so continuation tokens are exercised.
func seedLsBucket(t *testing.T) (*s3test.Server, *Session) {
	t.Helper()
	srv := s3test.New()
	t.Cleanup(srv.Close)
	srv.MaxListKeys = 2
	srv.CreateBucket(lsTestBucket)
	srv.AddObject(lsTestBucket, "2026/09/dump.sql", make([]byte, 1258291), "application/sql")
	srv.AddObject(lsTestBucket, "2026/09/notes.txt", []byte("notes"), "text/plain")
	srv.AddObject(lsTestBucket, "logs/app.log", []byte("log line"), "text/plain")
	srv.AddObject(lsTestBucket, "readme.md", []byte("# hi"), "text/markdown")
	srv.ResetRequests()
	return srv, newLsTestSession(t, srv, lsTestBucket)
}

func entryNames(entries []renderer.ResponseData) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		switch v := e.(type) {
		case objectstorage.Prefix:
			out = append(out, "PRE "+v.Prefix)
		case objectstorage.Object:
			out = append(out, v.Key)
		}
	}
	return out
}

func assertNoWrites(t *testing.T, srv *s3test.Server) {
	t.Helper()
	if w := srv.WriteRequests(); len(w) != 0 {
		t.Fatalf("ls issued %d write requests: %+v", len(w), w)
	}
}

func listRequests(srv *s3test.Server) []s3test.Request {
	var out []s3test.Request
	for _, r := range srv.Requests() {
		if r.Method == http.MethodGet && r.Query.Get("list-type") == "2" {
			out = append(out, r)
		}
	}
	return out
}

func TestListObjectsDelimiter(t *testing.T) {
	srv, sess := seedLsBucket(t)
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	got := strings.Join(entryNames(res.Entries), ",")
	want := "PRE 2026/,PRE logs/,readme.md"
	if got != want {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	if res.NextToken != "" {
		t.Fatalf("NextToken = %q, want empty (listing exhausted)", res.NextToken)
	}
	if res.Count != 1 || res.Bytes != 4 {
		t.Fatalf("summary = %d objects / %d bytes, want 1 / 4", res.Count, res.Bytes)
	}
	reqs := listRequests(srv)
	if len(reqs) < 2 {
		t.Fatalf("expected the listing to paginate with tokens, got %d list requests", len(reqs))
	}
	if reqs[1].Query.Get("continuation-token") == "" {
		t.Fatalf("second page did not carry a continuation token: %v", reqs[1].Query)
	}
	for _, r := range reqs {
		if r.Query.Get("delimiter") != "/" {
			t.Fatalf("delimiter = %q, want /", r.Query.Get("delimiter"))
		}
		if r.Query.Get("max-keys") != "1000" {
			t.Fatalf("max-keys = %q, want 1000 by default", r.Query.Get("max-keys"))
		}
	}
	assertNoWrites(t, srv)
}

func TestListObjectsRecursive(t *testing.T) {
	srv, sess := seedLsBucket(t)
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Recursive: true})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	got := strings.Join(entryNames(res.Entries), ",")
	want := "2026/09/dump.sql,2026/09/notes.txt,logs/app.log,readme.md"
	if got != want {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	if res.Count != 4 || res.Bytes != 1258291+5+8+4 {
		t.Fatalf("summary = %d objects / %d bytes", res.Count, res.Bytes)
	}
	for _, r := range listRequests(srv) {
		if r.Query.Get("delimiter") != "" {
			t.Fatalf("recursive listing sent delimiter %q", r.Query.Get("delimiter"))
		}
	}
	assertNoWrites(t, srv)
}

func TestListObjectsLiteralPrefix(t *testing.T) {
	_, sess := seedLsBucket(t)
	// No trailing slash is added: "2026" matches keys starting with "2026".
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Prefix: "2026"})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if got := strings.Join(entryNames(res.Entries), ","); got != "PRE 2026/" {
		t.Fatalf("entries = %q, want PRE 2026/", got)
	}
	res, err = listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Prefix: "2026/09/"})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if got := strings.Join(entryNames(res.Entries), ","); got != "2026/09/dump.sql,2026/09/notes.txt" {
		t.Fatalf("entries = %q", got)
	}
}

func TestListObjectsMaxItemsAndStartingToken(t *testing.T) {
	srv, sess := seedLsBucket(t)
	first, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Recursive: true, MaxItems: 3})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if len(first.Entries) != 3 {
		t.Fatalf("got %d entries, want 3 (max-items)", len(first.Entries))
	}
	if first.NextToken == "" {
		t.Fatalf("expected a NextToken when max-items stops early")
	}
	// max-items never over-fetches: the last request asks only for what is left.
	reqs := listRequests(srv)
	if last := reqs[len(reqs)-1]; last.Query.Get("max-keys") != "1" {
		t.Fatalf("last max-keys = %q, want 1", last.Query.Get("max-keys"))
	}

	rest, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Recursive: true, StartingToken: first.NextToken})
	if err != nil {
		t.Fatalf("listObjects (resume): %v", err)
	}
	if got := strings.Join(entryNames(rest.Entries), ","); got != "readme.md" {
		t.Fatalf("resumed entries = %q, want readme.md", got)
	}
	if rest.NextToken != "" {
		t.Fatalf("NextToken after the last page = %q", rest.NextToken)
	}
	assertNoWrites(t, srv)
}

func TestListObjectsNoPaginate(t *testing.T) {
	srv, sess := seedLsBucket(t)
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Recursive: true, NoPaginate: true, PageSize: 2})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("got %d entries, want one page of 2", len(res.Entries))
	}
	if res.NextToken == "" {
		t.Fatalf("expected a NextToken after one page")
	}
	if reqs := listRequests(srv); len(reqs) != 1 {
		t.Fatalf("--no-paginate issued %d list requests, want 1", len(reqs))
	}
}

func TestListObjectsStreamsEntries(t *testing.T) {
	_, sess := seedLsBucket(t)
	var buf bytes.Buffer
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{
		Recursive: true,
		OnEntry: func(e renderer.ResponseData) {
			if err := streamJSON(&buf, e); err != nil {
				t.Fatalf("streamJSON: %v", err)
			}
		},
	})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if len(res.Entries) != 0 {
		t.Fatalf("streamed entries must not be accumulated, got %d", len(res.Entries))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d NDJSON lines, want 4:\n%s", len(lines), buf.String())
	}
	var first map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 is not JSON: %v", err)
	}
	if first["key"] != "2026/09/dump.sql" || first["type"] != "object" {
		t.Fatalf("unexpected first line: %v", first)
	}
	if res.Count != 4 {
		t.Fatalf("Count = %d, want 4 even when streaming", res.Count)
	}
}

func TestListObjectsErrorsSurface(t *testing.T) {
	srv, sess := seedLsBucket(t)
	srv.DenyAll = true
	_, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{})
	if err == nil {
		t.Fatalf("expected an error from a denied listing")
	}
	herr := sess.humanize(err)
	if !strings.Contains(herr.Error(), "access denied") {
		t.Fatalf("humanized error = %q", herr)
	}
}

func TestListObjectsVersions(t *testing.T) {
	srv := s3test.New()
	t.Cleanup(srv.Close)
	b := srv.CreateBucket(lsTestBucket)
	b.Versioned = true
	srv.AddObject(lsTestBucket, "cfg/app.yaml", []byte("v1"), "text/yaml")
	srv.AddObject(lsTestBucket, "cfg/app.yaml", []byte("v2!"), "text/yaml")
	sess := newLsTestSession(t, srv, lsTestBucket)

	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Prefix: "cfg/", Recursive: true, Versions: true})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("got %d version entries, want 2: %v", len(res.Entries), entryNames(res.Entries))
	}
	// The fake server assigns version IDs before archiving the previous
	// version, so only the ordering, sizes and IsLatest are asserted here.
	latest, ok := res.Entries[0].(objectstorage.Object)
	if !ok || latest.Size != 3 || latest.VersionID == "" || latest.IsLatest == nil || !*latest.IsLatest {
		t.Fatalf("first entry should be the latest version (3 bytes): %+v", res.Entries[0])
	}
	older := res.Entries[1].(objectstorage.Object)
	if older.Size != 2 || older.VersionID == "" || older.IsLatest == nil || *older.IsLatest {
		t.Fatalf("second entry should be the older version (2 bytes): %+v", older)
	}

	// --max-items applies to version listings too.
	capped, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Recursive: true, Versions: true, MaxItems: 1})
	if err != nil {
		t.Fatalf("listObjects (capped): %v", err)
	}
	if len(capped.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(capped.Entries))
	}
	assertNoWrites(t, srv)
}

// TestLsObjectEntries lists a seeded prefix and checks the renderable entries
// (one PRE prefix + the objects with their sizes), which the shared renderer
// then prints as the table / json / csv the rest of the CLI uses.
func TestLsObjectEntries(t *testing.T) {
	_, sess := seedLsBucket(t)
	res, err := listObjects(context.Background(), sess.Client, lsTestBucket, listOptions{Prefix: "2026/09/"})
	if err != nil {
		t.Fatalf("listObjects: %v", err)
	}
	if res.Count != 2 || res.Bytes != 1258296 {
		t.Fatalf("count=%d bytes=%d, want 2 / 1258296", res.Count, res.Bytes)
	}
	var sizes []int64
	for _, e := range res.Entries {
		if o, ok := e.(objectstorage.Object); ok {
			sizes = append(sizes, o.Size)
		}
	}
	if len(sizes) != 2 || sizes[0] != 1258291 {
		t.Fatalf("object sizes = %v", sizes)
	}
}

// TestLsSummary checks the --summarize footer helper (used only in table mode).
func TestLsSummary(t *testing.T) {
	if got := objectstorage.LsSummary(3, 3*1024*1024, true); got != "\nTotal Objects: 3\n   Total Size: 3.0 MiB" {
		t.Fatalf("human summary = %q", got)
	}
	if got := objectstorage.LsSummary(2, 1258296, false); got != "\nTotal Objects: 2\n   Total Size: 1258296" {
		t.Fatalf("byte summary = %q", got)
	}
}

func TestBucketRowsSortedAndFiltered(t *testing.T) {
	created := time.Date(2026, 9, 7, 15, 12, 1, 0, time.Local)
	mk := func(id, name string, class components.StorageClass) components.ObjectStorageData {
		n := name
		i := id
		c := class
		t := created
		return components.ObjectStorageData{ID: &i, Attributes: &components.ObjectStorageDataAttributes{Name: &n, StorageClass: &c, CreatedAt: &t}}
	}
	data := []components.ObjectStorageData{
		mk("bkt_2", "logs", components.StorageClass("high_performance")),
		mk("bkt_1", "backups", components.StorageClass("standard")),
		mk("bkt_3", "media", components.StorageClass("standard")),
	}
	// BucketRows renders through the shared renderer; the row for a bucket keeps
	// its name so the table shows it.
	rows := BucketRows(data, nil)
	if len(rows) != 3 || rows[0].TableRow()["name"].Value != "logs" {
		t.Fatalf("unexpected rows: %d", len(rows))
	}

	// --storage-class goes through the shared alias table so "high-performance"
	// (and hp, vast…) mean the same thing here as in mb and access-keys.
	class, err := objectstorage.ParseStorageClass("high-performance")
	if err != nil || class != objectstorage.ClassHighPerformance {
		t.Fatalf("ParseStorageClass = %q, %v", class, err)
	}
	filtered := filterBucketsByClass(data, class)
	if len(filtered) != 1 || *filtered[0].Attributes.Name != "logs" {
		t.Fatalf("filtered = %v", entryBucketNames(filtered))
	}
	// An empty value is "no filter", not an error.
	if class, err := objectstorage.ParseStorageClass(""); err != nil || class != "" {
		t.Fatalf("ParseStorageClass(\"\") = %q, %v; want no filter", class, err)
	}
	if len(filterBucketsByClass(data, "")) != len(data) {
		t.Fatalf("an empty class must keep every bucket")
	}
	if _, err := objectstorage.ParseStorageClass("glacier"); exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("expected a usage error for an unknown storage class, got %v", err)
	}
}

// TestListObjectsCancelBetweenPages cancels the context while the first page
// is being consumed: the loop must stop before requesting the second page and
// surface context.Canceled, which the session maps to exit 130.
func TestListObjectsCancelBetweenPages(t *testing.T) {
	srv, sess := seedLsBucket(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var seen int
	res, err := listObjects(ctx, sess.Client, lsTestBucket, listOptions{
		Recursive: true,
		OnEntry: func(renderer.ResponseData) {
			seen++
			cancel()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if seen == 0 || seen > srv.MaxListKeys {
		t.Fatalf("saw %d entries, want only the first page (<= %d)", seen, srv.MaxListKeys)
	}
	if got := len(listRequests(srv)); got != 1 {
		t.Fatalf("issued %d list requests after cancellation, want 1", got)
	}
	if res.Total != int64(seen) {
		t.Fatalf("Total = %d, want %d", res.Total, seen)
	}
	if code := exitcode.Of(sess.humanize(err)); code != exitcode.Interrupted {
		t.Fatalf("humanized exit code = %d, want %d (interrupted)", code, exitcode.Interrupted)
	}
}

// TestListObjectsCancelDuringRequest cancels the context while a list request
// is still in flight: minio.Core.ListObjectsV2 has no context, so listObjects
// must stop waiting on its own instead of hanging until the server answers.
func TestListObjectsCancelDuringRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once bool
	blocking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !once {
			once = true
			close(started)
		}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(blocking.Close)

	b := &objectstorage.Bucket{ID: "bkt_1", Name: "backups", BucketName: lsTestBucket, Endpoint: blocking.URL, SigningRegion: "us-east-1"}
	cred := objectstorage.NewCredential("AK", "SK", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	sess := &Session{Bucket: b, Cred: cred, Client: client}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()
	done := make(chan error, 1)
	go func() {
		_, err := listObjects(ctx, sess.Client, lsTestBucket, listOptions{Recursive: true})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if code := exitcode.Of(sess.humanize(err)); code != exitcode.Interrupted {
			t.Fatalf("humanized exit code = %d, want %d (interrupted)", code, exitcode.Interrupted)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("listObjects did not return after the context was cancelled while the request was in flight")
	}
	// Let the abandoned request finish so the server can shut down cleanly.
	close(release)
}

// TestLsInvalidBackendNameExitsUsage drives the real openBucket path with an
// endpoint override: a backend bucket name S3 rejects ("b" is shorter than the
// three characters the protocol requires) must be a usage error (exit 2)
// before any request or credential lookup happens.
func TestLsInvalidBackendNameExitsUsage(t *testing.T) {
	srv := s3test.New()
	t.Cleanup(srv.Close)
	t.Setenv(objectstorage.EnvEndpointURL, srv.URL())

	cmd := NewLsCmd()
	err := runLsObjects(context.Background(), cmd, "s3://b/")
	if err == nil {
		t.Fatalf("expected an error for the one-letter backend bucket name")
	}
	if code := exitcode.Of(err); code != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d; err=%v", code, exitcode.Usage, err)
	}
	if !strings.Contains(err.Error(), "invalid bucket name") {
		t.Fatalf("error should name the invalid bucket: %v", err)
	}
	if reqs := srv.Requests(); len(reqs) != 0 {
		t.Fatalf("no request should reach the endpoint, got %d", len(reqs))
	}
}

func entryBucketNames(data []components.ObjectStorageData) []string {
	out := make([]string, 0, len(data))
	for _, d := range data {
		out = append(out, objectstorage.BucketFromData(d).Name)
	}
	return out
}
