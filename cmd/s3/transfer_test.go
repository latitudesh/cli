package s3

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

// newTestSession builds a Session against the fake server for bucket name.
func newTestSession(t *testing.T, srv *s3test.Server, name string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "b", BucketName: name, Endpoint: srv.URL(), StorageClass: "standard", SigningRegion: "us-east-1"}
	cred := objectstorage.NewCredential("AK", "SK", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

func remoteEP(sess *Session, key string) endpoint {
	raw := "s3://" + sess.Bucket.Name
	if key != "" {
		raw += "/" + key
	}
	return endpoint{Ref: objectstorage.Ref{Raw: raw, Bucket: sess.Bucket.Name, Key: key, Remote: true, HadScheme: true}, Sess: sess}
}

func localEP(path string) endpoint { return endpoint{Ref: objectstorage.Ref{Raw: path}} }

func stdioEP() endpoint { return endpoint{Ref: objectstorage.Ref{Raw: "-", Stdio: true}} }

func baseOpts() *transferOptions {
	return &transferOptions{Human: true, FollowSymlinks: true, Filters: &objectstorage.Filters{}}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run plans and executes a cp/mv between two endpoints.
func run(t *testing.T, src, dst endpoint, opts *transferOptions) ([]objectstorage.TransferResult, string, string, error) {
	t.Helper()
	ctx := context.Background()
	plan, err := buildPlan(ctx, src, dst, opts)
	if err != nil {
		return nil, "", "", err
	}
	var out, hints bytes.Buffer
	results, err := runTransfers(ctx, plan, opts, &out, &hints)
	return results, out.String(), hints.String(), err
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")

	// Local paths are displayed relative to the working directory (aws
	// behaviour), so the test runs inside the temp dir with relative operands.
	dir := t.TempDir()
	t.Chdir(dir)
	file := "report.json"
	writeFile(t, file, `{"ok":true}`)

	opts := baseOpts()
	opts.Metadata = map[string]string{"owner": "ops", "team": "infra"}
	opts.CacheControl = "max-age=60"
	opts.ContentLanguage = "en"
	results, out, _, err := run(t, localEP(file), remoteEP(sess, "2026/09/"), opts)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if want := "upload: ./report.json to s3://b/2026/09/report.json\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if len(results) != 1 || results[0].Op != "upload" || results[0].ContentType != "application/json" {
		t.Errorf("results = %+v", results)
	}
	obj := srv.Object("bucket-a", "2026/09/report.json")
	if obj == nil {
		t.Fatalf("object not stored; keys=%v", srv.Keys("bucket-a"))
	}
	if string(obj.Data) != `{"ok":true}` {
		t.Errorf("data = %q", obj.Data)
	}
	if obj.ContentType != "application/json" {
		t.Errorf("content type = %q, want application/json (guessed from extension)", obj.ContentType)
	}
	if obj.Metadata["owner"] != "ops" || obj.Metadata["team"] != "infra" {
		t.Errorf("metadata = %v", obj.Metadata)
	}
	if obj.Headers["Cache-Control"] != "max-age=60" || obj.Headers["Content-Language"] != "en" {
		t.Errorf("headers = %v", obj.Headers)
	}
	if srv.HasChecksumHeaders() {
		t.Error("upload must not send checksum trailers")
	}

	// Download into a directory (trailing separator appends the base name);
	// the destination is shown relative to cwd without "./" because it has a
	// directory component.
	dest := "restore" + string(filepath.Separator)
	results, out, _, err = run(t, remoteEP(sess, "2026/09/report.json"), localEP(dest), baseOpts())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if want := "download: s3://b/2026/09/report.json to " + filepath.Join("restore", "report.json") + "\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	got, err := os.ReadFile(filepath.Join(dir, "restore", "report.json"))
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("downloaded content = %q, err = %v", got, err)
	}
	if results[0].Size != int64(len(`{"ok":true}`)) || results[0].ContentType != "application/json" {
		t.Errorf("download result = %+v", results[0])
	}
	if _, err := os.Stat(filepath.Join(dir, "restore", ".report.json"+partialSuffix)); !os.IsNotExist(err) {
		t.Error("partial file must be renamed away")
	}
}

func TestContentTypeSniffAndNoGuess(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()

	// No extension: sniffed from content.
	html := filepath.Join(dir, "page")
	writeFile(t, html, "<!DOCTYPE html><html><body>hi</body></html>")
	if _, _, _, err := run(t, localEP(html), remoteEP(sess, ""), baseOpts()); err != nil {
		t.Fatal(err)
	}
	if ct := srv.Object("bucket-a", "page").ContentType; !strings.HasPrefix(ct, "text/html") {
		t.Errorf("sniffed content type = %q", ct)
	}

	// --no-guess-mime-type stores binary/octet-stream.
	opts := baseOpts()
	opts.NoGuessMime = true
	if _, _, _, err := run(t, localEP(html), remoteEP(sess, "raw"), opts); err != nil {
		t.Fatal(err)
	}
	if ct := srv.Object("bucket-a", "raw").ContentType; ct != "binary/octet-stream" {
		t.Errorf("no-guess content type = %q", ct)
	}

	// Explicit flag wins.
	opts = baseOpts()
	opts.ContentType = "application/x-custom"
	if _, _, _, err := run(t, localEP(html), remoteEP(sess, "custom"), opts); err != nil {
		t.Fatal(err)
	}
	if ct := srv.Object("bucket-a", "custom").ContentType; ct != "application/x-custom" {
		t.Errorf("explicit content type = %q", ct)
	}
}

// The destination rules from the aws user guide (and the spec table).
func TestDestinationRules(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "dir/file.txt", []byte("remote"), "text/plain")
	sess := newTestSession(t, srv, "bucket-a")

	// Run inside the temp dir: local display strings are relative to cwd.
	t.Chdir(t.TempDir())
	file := "file.txt"
	writeFile(t, file, "local")
	existing := "existing"
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	tree := "tree"
	writeFile(t, filepath.Join(tree, "a.txt"), "a")
	writeFile(t, filepath.Join(tree, "sub", "b.txt"), "b")

	cases := []struct {
		name      string
		src, dst  endpoint
		recursive bool
		wantKeys  []string // remote destination keys
		wantPaths []string // local destination paths
		wantDst   []string // display strings
	}{
		{"file to bucket root", localEP(file), remoteEP(sess, ""), false, []string{"file.txt"}, nil, []string{"s3://b/file.txt"}},
		{"file to prefix with slash", localEP(file), remoteEP(sess, "2026/09/"), false, []string{"2026/09/file.txt"}, nil, []string{"s3://b/2026/09/file.txt"}},
		{"file to literal key", localEP(file), remoteEP(sess, "2026/09"), false, []string{"2026/09"}, nil, []string{"s3://b/2026/09"}},
		{"file to renamed key", localEP(file), remoteEP(sess, "renamed.bin"), false, []string{"renamed.bin"}, nil, []string{"s3://b/renamed.bin"}},
		// Destinations with a directory component are shown without "./",
		// a bare file name with it (aws relpath rules).
		{"object to existing dir", remoteEP(sess, "dir/file.txt"), localEP(existing), false, nil, []string{filepath.Join(existing, "file.txt")}, []string{filepath.Join(existing, "file.txt")}},
		{"object to new dir with slash", remoteEP(sess, "dir/file.txt"), localEP("new/"), false, nil, []string{filepath.Join("new", "file.txt")}, []string{filepath.Join("new", "file.txt")}},
		{"object to literal file", remoteEP(sess, "dir/file.txt"), localEP("copy.txt"), false, nil, []string{"copy.txt"}, []string{"." + string(filepath.Separator) + "copy.txt"}},
		{"recursive dir to prefix without slash", localEP(tree), remoteEP(sess, "pre"), true, []string{"pre/a.txt", "pre/sub/b.txt"}, nil, []string{"s3://b/pre/a.txt", "s3://b/pre/sub/b.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseOpts()
			opts.Recursive = tc.recursive
			plan, err := buildPlan(context.Background(), tc.src, tc.dst, opts)
			if err != nil {
				t.Fatalf("buildPlan: %v", err)
			}
			var keys, paths, dsts []string
			for _, it := range plan.Items {
				if it.DstKey != "" {
					keys = append(keys, it.DstKey)
				}
				if it.DstPath != "" {
					paths = append(paths, it.DstPath)
				}
				dsts = append(dsts, it.Dst)
			}
			if strings.Join(keys, ",") != strings.Join(tc.wantKeys, ",") {
				t.Errorf("keys = %v, want %v", keys, tc.wantKeys)
			}
			if strings.Join(paths, ",") != strings.Join(tc.wantPaths, ",") {
				t.Errorf("paths = %v, want %v", paths, tc.wantPaths)
			}
			if strings.Join(dsts, ",") != strings.Join(tc.wantDst, ",") {
				t.Errorf("display = %v, want %v", dsts, tc.wantDst)
			}
		})
	}
}

func TestSingleObjectUsageErrors(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()

	// Directory without --recursive.
	if _, err := buildPlan(context.Background(), localEP(dir), remoteEP(sess, ""), baseOpts()); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("dir without --recursive: err = %v", err)
	}
	// Prefix source without --recursive.
	if _, err := buildPlan(context.Background(), remoteEP(sess, "pre/"), localEP(dir), baseOpts()); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("prefix without --recursive: err = %v", err)
	}
	// Missing object -> not found.
	if _, err := buildPlan(context.Background(), remoteEP(sess, "missing.txt"), localEP(dir), baseOpts()); exitcode.Of(err) != exitcode.NotFound {
		t.Errorf("missing object: err = %v", err)
	}
	// Missing local file -> not found.
	if _, err := buildPlan(context.Background(), localEP(filepath.Join(dir, "nope")), remoteEP(sess, ""), baseOpts()); exitcode.Of(err) != exitcode.NotFound {
		t.Errorf("missing file: err = %v", err)
	}
	// Recursive download refuses keys escaping the destination.
	srv.AddObject("bucket-a", "pre/../../etc/passwd", []byte("x"), "")
	opts := baseOpts()
	opts.Recursive = true
	if _, err := buildPlan(context.Background(), remoteEP(sess, "pre/"), localEP(dir), opts); exitcode.Of(err) != exitcode.Refused {
		t.Errorf("escaping key: err = %v", err)
	}
	// Single-object download into a directory applies the same guard to the
	// appended name (exit 7); a literal file destination never uses the name.
	for _, name := range []string{"..", ".", "sub/x", "../x"} {
		if _, _, err := localDestination(dir, name); exitcode.Of(err) != exitcode.Refused || !strings.Contains(err.Error(), "escapes the destination directory") {
			t.Errorf("localDestination(dir, %q): err = %v", name, err)
		}
	}
	if _, _, err := localDestination(filepath.Join(dir, "literal.txt"), ".."); err != nil {
		t.Errorf("literal destination must not check the name: %v", err)
	}
	srv.AddObject("bucket-a", "pre/..", []byte("x"), "")
	if _, err := buildPlan(context.Background(), remoteEP(sess, "pre/.."), localEP(dir), baseOpts()); exitcode.Of(err) != exitcode.Refused {
		t.Errorf("single download of an escaping key: err = %v", err)
	}
}

func TestClassifyOperands(t *testing.T) {
	if _, _, err := classifyOperands("./a", "./b", false, false); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("local->local: %v", err)
	}
	if _, _, err := classifyOperands("-", "./b", false, false); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("stdin->local: %v", err)
	}
	if _, _, err := classifyOperands("-", "s3://b/k", true, false); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("mv from stdin: %v", err)
	}
	if _, _, err := classifyOperands("./a", "s3://b/", false, true); err != nil {
		t.Errorf("sync local->remote: %v", err)
	}
	src, dst, err := classifyOperands("s3://b/k", "-", false, false)
	if err != nil || !src.Remote || !dst.Stdio {
		t.Errorf("remote->stdout: %v %+v %+v", err, src, dst)
	}
}

func TestNoOverwrite(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	writeFile(t, file, "v1")

	opts := baseOpts()
	opts.NoOverwrite = true
	if _, _, _, err := run(t, localEP(file), remoteEP(sess, ""), opts); err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, "v2")
	srv.ResetRequests()
	results, out, hints, err := run(t, localEP(file), remoteEP(sess, ""), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 || out != "" {
		t.Errorf("skipped transfer must produce no result/line: %+v %q", results, out)
	}
	if !strings.Contains(hints, "skip: s3://b/f.txt already exists") {
		t.Errorf("hints = %q", hints)
	}
	if n := len(srv.WriteRequests()); n != 0 {
		t.Errorf("no-overwrite issued %d write requests", n)
	}
	if string(srv.Object("bucket-a", "f.txt").Data) != "v1" {
		t.Error("object was overwritten")
	}

	// Local destination is honoured too.
	dest := filepath.Join(dir, "out.txt")
	writeFile(t, dest, "keep")
	if _, _, hints, err = run(t, remoteEP(sess, "f.txt"), localEP(dest), opts); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "keep" || !strings.Contains(hints, "skip: ") {
		t.Errorf("local file overwritten: %q hints=%q", got, hints)
	}
}

func TestDryRunMakesNoWrites(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "pre/x.txt", []byte("x"), "")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	t.Chdir(dir) // display strings are relative to cwd
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), "b")

	opts := baseOpts()
	opts.Recursive, opts.DryRun = true, true
	results, out, _, err := run(t, localEP("."), remoteEP(sess, "pre/"), opts)
	if err != nil {
		t.Fatal(err)
	}
	want := "(dryrun) upload: ./a.txt to s3://b/pre/a.txt\n" +
		"(dryrun) upload: " + filepath.Join("sub", "b.txt") + " to s3://b/pre/sub/b.txt\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if len(results) != 2 || !results[0].DryRun {
		t.Errorf("results = %+v", results)
	}
	// Dry-run download and mv too.
	opts.Move = true
	if _, out, _, err = run(t, remoteEP(sess, "pre/"), localEP(filepath.Join(dir, "down")), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "(dryrun) move: s3://b/pre/x.txt to ") {
		t.Errorf("stdout = %q", out)
	}
	if srv.Object("bucket-a", "pre/x.txt") == nil {
		t.Error("dry-run mv deleted the source")
	}
	if _, err := os.Stat(filepath.Join(dir, "down")); !os.IsNotExist(err) {
		t.Error("dry-run created the destination directory")
	}
	if w := srv.WriteRequests(); len(w) != 0 {
		t.Errorf("dry-run issued %d write requests: %+v", len(w), w[0].Method+" "+w[0].Path)
	}
}

func TestRecursiveUploadWithFilters(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.log"), "a")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	writeFile(t, filepath.Join(dir, "sub", "c.log"), "c")
	writeFile(t, filepath.Join(dir, "sub", "d.tmp"), "d")

	opts := baseOpts()
	opts.Recursive = true
	_ = opts.Filters.Add(false, "*")
	_ = opts.Filters.Add(true, "*.log")
	results, out, _, err := run(t, localEP(dir), remoteEP(sess, "logs/"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(srv.Keys("bucket-a"), ","); got != "logs/a.log,logs/sub/c.log" {
		t.Errorf("keys = %s", got)
	}
	if len(results) != 2 || strings.Count(out, "upload: ") != 2 {
		t.Errorf("results = %d, out = %q", len(results), out)
	}
}

func TestQuietSuppressesLines(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	file := filepath.Join(t.TempDir(), "q.txt")
	writeFile(t, file, "q")
	opts := baseOpts()
	opts.Quiet = true
	results, out, _, err := run(t, localEP(file), remoteEP(sess, ""), opts)
	if err != nil || out != "" || len(results) != 1 {
		t.Errorf("quiet: err=%v out=%q results=%d", err, out, len(results))
	}
}

func TestStdoutDestinationWritesOnlyPayload(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	payload := strings.Repeat("payload-", 1000)
	srv.AddObject("bucket-a", "big.bin", []byte(payload), "application/octet-stream")
	sess := newTestSession(t, srv, "bucket-a")

	var stdout bytes.Buffer
	opts := baseOpts()
	opts.Stdout = &stdout
	opts.ShowProgress = true // must go to hints, never stdout
	results, out, _, err := run(t, remoteEP(sess, "big.bin"), stdioEP(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != payload {
		t.Errorf("stdout payload mismatch (len %d)", stdout.Len())
	}
	if out != "" {
		t.Errorf("no lines may be printed on stdout when the destination is '-': %q", out)
	}
	if len(results) != 1 || results[0].Destination != "-" {
		t.Errorf("results = %+v", results)
	}
}

func TestStdinUpload(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")

	opts := baseOpts()
	opts.Stdin = strings.NewReader("hello from stdin\n")
	results, out, hints, err := run(t, stdioEP(), remoteEP(sess, "in/notes.txt"), opts)
	if err != nil {
		t.Fatal(err)
	}
	obj := srv.Object("bucket-a", "in/notes.txt")
	if obj == nil || string(obj.Data) != "hello from stdin\n" {
		t.Fatalf("object = %+v", obj)
	}
	if !strings.HasPrefix(obj.ContentType, "text/plain") {
		t.Errorf("content type = %q", obj.ContentType)
	}
	if out != "upload: - to s3://b/in/notes.txt\n" || len(results) != 1 {
		t.Errorf("out = %q results=%+v", out, results)
	}
	if !strings.Contains(hints, "streaming from stdin") {
		t.Errorf("expected memory note on stderr, got %q", hints)
	}
	// A prefix destination is a usage error for stdin.
	if _, err := buildPlan(context.Background(), stdioEP(), remoteEP(sess, "in/"), baseOpts()); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("stdin to prefix: %v", err)
	}
}

// --expected-size is a hint for part sizing and the progress total only: the
// upload must store the whole stream whether it is shorter or longer than the
// flag (minio would otherwise fail on a short stream and truncate a long one).
func TestStdinUploadExpectedSizeIsOnlyAHint(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	payload := strings.Repeat("0123456789", 4) // 40 bytes

	for _, expected := range []int64{5, 1 << 30} {
		opts := baseOpts()
		opts.Stdin = strings.NewReader(payload)
		opts.ExpectedSize = expected
		key := "in/hint.bin"
		results, _, hints, err := run(t, stdioEP(), remoteEP(sess, key), opts)
		if err != nil {
			t.Fatalf("expected-size %d: %v", expected, err)
		}
		obj := srv.Object("bucket-a", key)
		if obj == nil || string(obj.Data) != payload {
			t.Fatalf("expected-size %d: stored %q, want the full %d-byte payload", expected, obj.Data, len(payload))
		}
		if len(results) != 1 || results[0].Size != int64(len(payload)) {
			t.Errorf("expected-size %d: result size = %+v, want %d", expected, results, len(payload))
		}
		if strings.Contains(hints, "streaming from stdin") {
			t.Errorf("expected-size %d: the part-size note must only appear without --expected-size: %q", expected, hints)
		}
	}

	// Part sizing: minio's default up to 160 GiB, larger multiples beyond.
	if got := stdinPartSizeFor(0); got != stdinPartSize {
		t.Errorf("stdinPartSizeFor(0) = %d", got)
	}
	if got := stdinPartSizeFor(5); got != stdinPartSize {
		t.Errorf("stdinPartSizeFor(5) = %d", got)
	}
	if got := stdinPartSizeFor(stdinMaxPlainSize); got != stdinPartSize {
		t.Errorf("stdinPartSizeFor(160 GiB) = %d", got)
	}
	if got := stdinPartSizeFor(1 << 40); got <= stdinPartSize || got%stdinPartSize != 0 || got*stdinMaxParts < 1<<40 {
		t.Errorf("stdinPartSizeFor(1 TiB) = %d, want a multiple of 16 MiB that fits 10000 parts", got)
	}
}

func TestMvRemovesSource(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	t.Chdir(t.TempDir()) // display strings are relative to cwd
	file := "m.txt"
	writeFile(t, file, "move me")

	opts := baseOpts()
	opts.Move = true
	_, out, _, err := run(t, localEP(file), remoteEP(sess, "moved/"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if out != "move: ./m.txt to s3://b/moved/m.txt\n" {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("local source must be removed after a successful upload")
	}
	if srv.Object("bucket-a", "moved/m.txt") == nil {
		t.Fatal("object missing")
	}

	// Remote -> remote move within the bucket (server-side copy + delete).
	_, out, _, err = run(t, remoteEP(sess, "moved/m.txt"), remoteEP(sess, "archive/"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if out != "move: s3://b/moved/m.txt to s3://b/archive/m.txt\n" {
		t.Errorf("out = %q", out)
	}
	if srv.Object("bucket-a", "moved/m.txt") != nil || srv.Object("bucket-a", "archive/m.txt") == nil {
		t.Errorf("keys after move = %v", srv.Keys("bucket-a"))
	}
	if string(srv.Object("bucket-a", "archive/m.txt").Data) != "move me" {
		t.Error("copied data mismatch")
	}

	// Same object is refused.
	if _, err := buildPlan(context.Background(), remoteEP(sess, "archive/m.txt"), remoteEP(sess, "archive/"), opts); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("same object: %v", err)
	}
}

func TestMvKeepsSourceWhenCopyFails(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	file := filepath.Join(t.TempDir(), "keep.txt")
	writeFile(t, file, "keep")

	opts := baseOpts()
	opts.Move = true
	srv.FailNext = &s3test.ErrorResponse{Code: "AccessDenied", Message: "Access Denied", Status: http.StatusForbidden}
	results, _, hints, err := run(t, localEP(file), remoteEP(sess, "x/"), opts)
	if exitcode.Of(err) != exitcode.Permission {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(file); statErr != nil {
		t.Error("source must survive a failed copy")
	}
	// Single-item plans return the error (printed once by the command) and
	// do not duplicate it as a failure line.
	if strings.Contains(hints, "move failed: ") || len(results) != 1 || results[0].Error == "" {
		t.Errorf("hints = %q results = %+v", hints, results)
	}
}

func TestCrossEndpointCopyRefused(t *testing.T) {
	srvA, srvB := s3test.New(), s3test.New()
	defer srvA.Close()
	defer srvB.Close()
	srvA.CreateBucket("bucket-a")
	srvB.CreateBucket("bucket-b")
	a, b := newTestSession(t, srvA, "bucket-a"), newTestSession(t, srvB, "bucket-b")
	_, err := buildPlan(context.Background(), remoteEP(a, "k"), remoteEP(b, "k"), baseOpts())
	if exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "configure export --format rclone") {
		t.Errorf("err = %v", err)
	}
}

func TestRecursiveFailuresContinue(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "1.txt"), "1")
	writeFile(t, filepath.Join(dir, "2.txt"), "2")

	opts := baseOpts()
	opts.Recursive = true
	srv.FailNext = &s3test.ErrorResponse{Code: "InternalError", Message: "boom", Status: 500}
	results, out, hints, err := run(t, localEP(dir), remoteEP(sess, ""), opts)
	if exitcode.Of(err) != exitcode.Generic || !strings.Contains(err.Error(), "1 of 2 transfers failed") {
		t.Errorf("err = %v", err)
	}
	if len(results) != 2 || results[0].Error == "" || results[1].Error != "" {
		t.Errorf("results = %+v", results)
	}
	if !strings.Contains(hints, "upload failed: ") || strings.Count(out, "upload: ") != 1 {
		t.Errorf("out=%q hints=%q", out, hints)
	}
}

func TestSyncPlan(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new.txt"), "new")
	writeFile(t, filepath.Join(dir, "same.txt"), "same")
	writeFile(t, filepath.Join(dir, "bigger.txt"), "bigger now")
	writeFile(t, filepath.Join(dir, "skip.tmp"), "tmp")
	old := time.Now().Add(-2 * time.Hour)
	for _, n := range []string{"same.txt", "bigger.txt"} {
		if err := os.Chtimes(filepath.Join(dir, n), old, old); err != nil {
			t.Fatal(err)
		}
	}
	srv.AddObject("bucket-a", "site/same.txt", []byte("same"), "")
	srv.AddObject("bucket-a", "site/bigger.txt", []byte("small"), "")
	srv.AddObject("bucket-a", "site/stale.txt", []byte("stale"), "")
	srv.AddObject("bucket-a", "site/keep.tmp", []byte("keep"), "")

	opts := baseOpts()
	opts.Recursive = true
	_ = opts.Filters.Add(false, "*.tmp")
	plan, err := buildSyncPlan(context.Background(), localEP(dir), remoteEP(sess, "site"), opts, syncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, it := range plan.Items {
		keys = append(keys, it.DstKey)
	}
	if got := strings.Join(keys, ","); got != "site/bigger.txt,site/new.txt" {
		t.Errorf("planned uploads = %s", got)
	}
	if len(plan.Deletes) != 1 || plan.Deletes[0].Key != "site/stale.txt" {
		t.Errorf("planned deletes = %+v (filters must protect keep.tmp)", plan.Deletes)
	}

	var out, hints bytes.Buffer
	results, err := runTransfers(context.Background(), plan, opts, &out, &hints)
	if err != nil {
		t.Fatalf("run: %v (%s)", err, hints.String())
	}
	if got := strings.Join(srv.Keys("bucket-a"), ","); got != "site/bigger.txt,site/keep.tmp,site/new.txt,site/same.txt" {
		t.Errorf("keys after sync = %s", got)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "upload: ") || lines[2] != "delete: s3://b/site/stale.txt" {
		t.Errorf("lines = %q", lines)
	}
	if len(results) != 3 || results[2].Op != "delete" {
		t.Errorf("results = %+v", results)
	}

	// Second run: nothing to do.
	plan, err = buildSyncPlan(context.Background(), localEP(dir), remoteEP(sess, "site"), opts, syncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 0 || len(plan.Deletes) != 0 {
		t.Errorf("second sync must be a no-op: items=%+v deletes=%+v", plan.Items, plan.Deletes)
	}

	// Size-only ignores a newer source of the same size.
	writeFile(t, filepath.Join(dir, "same.txt"), "SAME")
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "same.txt"), future, future)
	plan, _ = buildSyncPlan(context.Background(), localEP(dir), remoteEP(sess, "site"), opts, syncOptions{SizeOnly: true})
	if len(plan.Items) != 0 {
		t.Errorf("--size-only planned %+v", plan.Items)
	}
	plan, _ = buildSyncPlan(context.Background(), localEP(dir), remoteEP(sess, "site"), opts, syncOptions{})
	if len(plan.Items) != 1 || plan.Items[0].DstKey != "site/same.txt" {
		t.Errorf("newer source must be uploaded: %+v", plan.Items)
	}
}

func TestSyncDownloadDryRunAndDelete(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "data/a.txt", []byte("a"), "")
	srv.AddObject("bucket-a", "data/sub/b.txt", []byte("b"), "")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	t.Chdir(dir) // display strings are relative to cwd
	writeFile(t, filepath.Join(dir, "extra.txt"), "extra")

	opts := baseOpts()
	opts.Recursive, opts.DryRun = true, true
	plan, err := buildSyncPlan(context.Background(), remoteEP(sess, "data/"), localEP("."), opts, syncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	var out, hints bytes.Buffer
	if _, err := runTransfers(context.Background(), plan, opts, &out, &hints); err != nil {
		t.Fatal(err)
	}
	want := "(dryrun) download: s3://b/data/a.txt to ./a.txt\n" +
		"(dryrun) download: s3://b/data/sub/b.txt to " + filepath.Join("sub", "b.txt") + "\n" +
		"(dryrun) delete: ./extra.txt\n"
	if out.String() != want {
		t.Errorf("out = %q, want %q", out.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); err != nil {
		t.Error("dry-run deleted a local file")
	}

	opts.DryRun = false
	out.Reset()
	if _, err := runTransfers(context.Background(), plan, opts, &out, &hints); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "sub", "b.txt")); string(got) != "b" {
		t.Errorf("downloaded b.txt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); !os.IsNotExist(err) {
		t.Error("--delete must remove local extras")
	}
	// Downloaded files carry the object's mtime so the next sync is a no-op.
	plan, err = buildSyncPlan(context.Background(), remoteEP(sess, "data/"), localEP("."), opts, syncOptions{Delete: true, ExactTimestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 0 || len(plan.Deletes) != 0 {
		t.Errorf("re-sync must be a no-op: %+v %+v", plan.Items, plan.Deletes)
	}
}

func TestParseMetadataAndExpires(t *testing.T) {
	m, err := parseMetadata([]string{"a=1,b=2", "c=x=y"})
	if err != nil || m["a"] != "1" || m["b"] != "2" || m["c"] != "x=y" {
		t.Errorf("metadata = %v err=%v", m, err)
	}
	if _, err := parseMetadata([]string{"novalue"}); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("invalid metadata: %v", err)
	}
	if ts, err := parseExpires("2026-09-07"); err != nil || ts.Year() != 2026 || ts.Day() != 7 {
		t.Errorf("date expires = %v %v", ts, err)
	}
	if _, err := parseExpires("2026-09-07T10:00:00Z"); err != nil {
		t.Errorf("rfc3339 expires: %v", err)
	}
	if _, err := parseExpires("tomorrow"); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("bad expires: %v", err)
	}
}

// displayLocal follows aws (os.path.relpath from cwd): "./" only for a bare
// file name, directory components as-is, absolute paths made relative.
func TestDisplayLocal(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	cases := map[string]string{
		"dump.sql":                            "." + sep + "dump.sql",
		"." + sep + "dump.sql":                "." + sep + "dump.sql",
		filepath.Join("sub", "b.txt"):         filepath.Join("sub", "b.txt"),
		filepath.Join("..", "x"):              filepath.Join("..", "x"),
		filepath.Join(cwd, "dump.sql"):        "." + sep + "dump.sql",
		filepath.Join(cwd, "sub", "x"):        filepath.Join("sub", "x"),
		filepath.Join(filepath.Dir(cwd), "o"): filepath.Join("..", "o"),
		".":                                   ".",
	}
	for in, want := range cases {
		if got := displayLocal(in); got != want {
			t.Errorf("displayLocal(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommandsRegisterFlags(t *testing.T) {
	cp, mv, sync := NewCpCmd(), NewMvCmd(), NewSyncCmd()
	for _, name := range []string{"recursive", "exclude", "include", "content-type", "metadata", "no-overwrite", "expected-size", "quiet", "only-show-errors", "no-progress", "follow-symlinks", "no-follow-symlinks", "project", "storage-class", "acl", "sse", "region"} {
		if cp.Flags().Lookup(name) == nil {
			t.Errorf("cp lacks --%s", name)
		}
		if mv.Flags().Lookup(name) == nil {
			t.Errorf("mv lacks --%s", name)
		}
	}
	for _, name := range []string{"delete", "size-only", "exact-timestamps", "exclude", "include", "project", "region"} {
		if sync.Flags().Lookup(name) == nil {
			t.Errorf("sync lacks --%s", name)
		}
	}
	// `aws s3 cp ... --region x` gets the directed explanation (exit 2) on
	// every transfer command.
	for _, c := range []*cobra.Command{mv, sync} {
		if err := c.Flags().Set("region", "us-east-1"); err != nil {
			t.Fatal(err)
		}
		err := c.PreRunE(c, nil)
		if exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "--signing-region") {
			t.Errorf("%s --region: %v", c.Name(), err)
		}
	}
	if sync.Flags().Lookup("recursive") != nil {
		t.Error("sync is always recursive and must not expose --recursive")
	}
	if !strings.Contains(cp.Long, "2026/09") || !strings.Contains(mv.Long, "2026/09") {
		t.Error("destination rules must be documented in the help text")
	}
	// --storage-class is rejected with the Latitude explanation.
	if err := cp.Flags().Set("storage-class", "STANDARD_IA"); err != nil {
		t.Fatal(err)
	}
	err := cp.PreRunE(cp, nil)
	if exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "lsh s3 mb --storage-class") {
		t.Errorf("--storage-class: %v", err)
	}
}

func TestMultipartUploadWithProgress(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	sess := newTestSession(t, srv, "bucket-a")
	t.Chdir(t.TempDir()) // display strings are relative to cwd
	file := "big.bin"
	data := bytes.Repeat([]byte{0xAB}, 17<<20) // above minio's 16 MiB part size
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	opts := baseOpts()
	opts.ShowProgress = true
	results, out, hints, err := run(t, localEP(file), remoteEP(sess, "big.bin"), opts)
	if err != nil {
		t.Fatal(err)
	}
	obj := srv.Object("bucket-a", "big.bin")
	if obj == nil || len(obj.Data) != len(data) || !bytes.Equal(obj.Data, data) {
		t.Fatalf("multipart object mismatch (len %d)", len(obj.Data))
	}
	if results[0].Size != int64(len(data)) || out != "upload: ./big.bin to s3://b/big.bin\n" {
		t.Errorf("results = %+v out = %q", results, out)
	}
	if !strings.Contains(hints, "%)") || !strings.HasSuffix(hints, "\r\033[K") {
		t.Errorf("progress must be drawn and cleared on hints: %q", hints)
	}
	if srv.HasChecksumHeaders() {
		t.Error("multipart upload must not send checksum trailers")
	}
}

// TestSyncDeleteSkippedAfterFailure locks the sync safety rule: when a
// transfer fails, the planned deletions are not applied, so the destination is
// never left without both the object that failed and the one being pruned.
func TestSyncDeleteSkippedAfterFailure(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "data/a.txt", []byte("a"), "")
	sess := newTestSession(t, srv, "bucket-a")
	dir := t.TempDir()
	t.Chdir(dir) // display strings are relative to cwd
	writeFile(t, filepath.Join(dir, "extra.txt"), "extra")
	// A directory where the download must write its file fails the transfer
	// without involving the fake backend.
	if err := os.MkdirAll(filepath.Join(dir, "a.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := baseOpts()
	opts.Recursive = true
	plan, err := buildSyncPlan(context.Background(), remoteEP(sess, "data/"), localEP("."), opts, syncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("planned items = %+v, want the one download", plan.Items)
	}
	var planned []string
	for _, d := range plan.Deletes {
		planned = append(planned, d.Path)
	}
	if len(plan.Deletes) == 0 {
		t.Fatalf("expected extra.txt to be planned for deletion, got %v", planned)
	}

	var out, hints bytes.Buffer
	results, err := runTransfers(context.Background(), plan, opts, &out, &hints)
	if err == nil {
		t.Fatal("expected the failed download to be reported")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "extra.txt")); statErr != nil {
		t.Errorf("a failed sync must not apply its deletions: %v", statErr)
	}
	if !strings.Contains(hints.String(), "not applied because") {
		t.Errorf("hints = %q, want the skipped-deletions warning", hints.String())
	}
	// Skipping the deletions must not turn the run into a single-item plan:
	// the failure was already reported on hints, so returning the item's own
	// error would make the command wrapper print it a second time.
	if !strings.Contains(err.Error(), "transfers failed") {
		t.Errorf("err = %v, want the summary error (the failure is already on stderr)", err)
	}
	if n := strings.Count(hints.String(), "download failed:"); n != 1 {
		t.Errorf("the failure must be reported once, found %d times:\n%s", n, hints.String())
	}
	for _, r := range results {
		if r.Op == opDelete {
			t.Errorf("delete was executed after a failure: %+v", r)
		}
	}
}

// TestSyncDeleteStaysInsideDestination locks the containment rule for
// `sync --delete` on a local destination: the enumeration follows directory
// symlinks, so a deletion candidate reached through one resolves outside the
// tree the user pointed at and must not be removed.
func TestSyncDeleteStaysInsideDestination(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "data/keep.txt", []byte("keep"), "")
	sess := newTestSession(t, srv, "bucket-a")

	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "do not delete me")
	dst := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dst, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// A genuine destination-only file is the positive control.
	writeFile(t, filepath.Join(dst, "stale.txt"), "stale")

	opts := baseOpts()
	opts.Recursive = true
	if !opts.FollowSymlinks {
		t.Fatal("this test needs symlink following enabled")
	}
	plan, err := buildSyncPlan(context.Background(), remoteEP(sess, "data/"), localEP(dst), opts, syncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	var planned []string
	for _, d := range plan.Deletes {
		planned = append(planned, d.Path)
		if strings.Contains(d.Path, "link") || strings.HasPrefix(d.Path, outside) {
			t.Errorf("planned a deletion outside the destination: %s", d.Path)
		}
	}
	if len(plan.Deletes) != 1 || !strings.HasSuffix(plan.Deletes[0].Path, "stale.txt") {
		t.Fatalf("planned deletes = %v, want only stale.txt", planned)
	}

	var out, hints bytes.Buffer
	if _, err := runTransfers(context.Background(), plan, opts, &out, &hints); err != nil {
		t.Fatalf("run: %v (%s)", err, hints.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.txt")); err != nil {
		t.Errorf("a file outside the destination was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("the destination-only file should have been deleted: %v", err)
	}
}

func TestDeleteInsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "a.txt"), true},
		{filepath.Join(root, "sub", "a.txt"), true},
		// The link itself lives in the root: removing it only drops the link.
		{filepath.Join(root, "link"), true},
		{filepath.Join(root, "link", "a.txt"), false},
		{filepath.Join(outside, "a.txt"), false},
		// A parent that cannot be resolved is never deleted through.
		{filepath.Join(root, "missing", "a.txt"), false},
	}
	for _, c := range cases {
		if got := deleteInsideRoot(resolved, c.path); got != c.want {
			t.Errorf("deleteInsideRoot(%s) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestProgressThrottleWithUnknownTotal covers the throttle fix: with total 0
// (a stdin upload) the "final frame" condition was always false, so every
// transport chunk redrew the stderr line.
func TestProgressThrottleWithUnknownTotal(t *testing.T) {
	var buf bytes.Buffer
	p := newProgressMeter(&buf, "upload", 0)
	for i := 0; i < 50; i++ {
		p.add(1024)
	}
	if n := strings.Count(buf.String(), "\r"); n > 1 {
		t.Errorf("redrew the line %d times for 50 chunks; the throttle must apply when the total is unknown:\n%q", n, buf.String())
	}
	// A known total still forces the last frame.
	buf.Reset()
	q := newProgressMeter(&buf, "upload", 2048)
	q.add(1024)
	q.add(1024)
	if !strings.Contains(buf.String(), "upload") {
		t.Errorf("a completed transfer must render its final frame, got %q", buf.String())
	}
}

// TestDryRunToStdoutTargetReportsOnStderr covers `cp s3://b/key - --dryrun`:
// stdout carries the object, so the plan line has to go to stderr instead of
// being swallowed entirely.
func TestDryRunToStdoutTargetReportsOnStderr(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("bucket-a")
	srv.AddObject("bucket-a", "readme.txt", []byte("hi"), "text/plain")
	sess := newTestSession(t, srv, "bucket-a")

	opts := baseOpts()
	opts.DryRun = true
	_, out, hints, err := run(t, remoteEP(sess, "readme.txt"), stdioEP(), opts)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if out != "" {
		t.Errorf("stdout must stay empty for a '-' destination, got %q", out)
	}
	if !strings.Contains(hints, "(dryrun)") || !strings.Contains(hints, "readme.txt") {
		t.Errorf("the plan must be reported on stderr, got %q", hints)
	}
}

// newTestSessionAs builds a session for one bucket with a named credential, so
// a test can hold two sessions with different access keys on one endpoint.
func newTestSessionAs(t *testing.T, srv *s3test.Server, display, backend, keyID string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{ID: "bkt_" + backend, Name: display, BucketName: backend, Endpoint: srv.URL(), StorageClass: "standard", SigningRegion: "us-east-1"}
	cred := objectstorage.NewCredential(keyID, "SK", "profile")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

// TestCopyStreamsWhenCredentialsDiffer covers the cross-key fallback: a
// server-side CopyObject carries one identity, so when the two buckets resolved
// different keys the destination key cannot read the source. The object is
// streamed through the client instead of failing with an access denial.
func TestCopyStreamsWhenCredentialsDiffer(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("source-bucket")
	srv.CreateBucket("target-bucket")
	source := srv.AddObject("source-bucket", "data.txt", []byte("payload"), "text/plain")
	source.Metadata = map[string]string{"owner": "ops"}
	source.Headers = map[string]string{"Cache-Control": "max-age=60"}

	srcSess := newTestSessionAs(t, srv, "logs", "source-bucket", "AK1")
	dstSess := newTestSessionAs(t, srv, "backups", "target-bucket", "AK2")
	opts := baseOpts()
	src, dst := remoteEP(srcSess, "data.txt"), remoteEP(dstSess, "data.txt")
	plan, err := buildPlan(context.Background(), src, dst, opts)
	if err != nil {
		t.Fatal(err)
	}
	// Refuse the server-side copy only, as the backend would when the
	// destination key has no read permission on the source bucket. Planning
	// already happened, so the failure lands on CopyObject.
	srv.FailNext = &s3test.ErrorResponse{Code: "AccessDenied", Message: "Access Denied", Status: 403}

	var outBuf, hintsBuf bytes.Buffer
	results, err := runTransfers(context.Background(), plan, opts, &outBuf, &hintsBuf)
	hints := hintsBuf.String()
	if err != nil {
		t.Fatalf("copy across credentials: %v (%s)", err, hints)
	}
	obj := srv.Object("target-bucket", "data.txt")
	if obj == nil {
		t.Fatalf("the destination object was not written; keys=%v", srv.Keys("target-bucket"))
	}
	if string(obj.Data) != "payload" {
		t.Errorf("destination object = %q, want the streamed payload", obj.Data)
	}
	// A streamed copy must keep what a server-side copy would have kept.
	if obj.ContentType != "text/plain" {
		t.Errorf("content type = %q, want the source's", obj.ContentType)
	}
	if obj.Metadata["owner"] != "ops" {
		t.Errorf("user metadata = %v, want the source's owner=ops", obj.Metadata)
	}
	if obj.Headers["Cache-Control"] != "max-age=60" {
		t.Errorf("headers = %v, want the source's Cache-Control", obj.Headers)
	}
	if !strings.Contains(hints, "different access keys") {
		t.Errorf("the fallback must say why it streams:\n%s", hints)
	}
	if len(results) != 1 || results[0].Error != "" {
		t.Errorf("results = %+v, want one successful copy", results)
	}
}

// TestNeedsStreamedCopy keeps the fallback narrow: only an authorization
// failure between two different credentials qualifies.
func TestNeedsStreamedCopy(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("alpha-bucket")
	srv.CreateBucket("beta-bucket")
	src := remoteEP(newTestSessionAs(t, srv, "src", "alpha-bucket", "AK1"), "k")
	dstOther := remoteEP(newTestSessionAs(t, srv, "dst", "beta-bucket", "AK2"), "k")
	dstSame := remoteEP(newTestSessionAs(t, srv, "dst", "beta-bucket", "AK1"), "k")
	denied := minio.ErrorResponse{Code: "AccessDenied", StatusCode: 403}
	missing := minio.ErrorResponse{Code: "NoSuchKey", StatusCode: 404}

	if !needsStreamedCopy(src, dstOther, denied) {
		t.Error("access denied across two credentials must stream")
	}
	if needsStreamedCopy(src, dstSame, denied) {
		t.Error("the same credential cannot be fixed by streaming")
	}
	if needsStreamedCopy(src, dstOther, missing) {
		t.Error("a missing object would fail the same way twice")
	}
	if needsStreamedCopy(localEP("./f"), dstOther, denied) {
		t.Error("only remote -> remote copies use CopyObject")
	}
}
