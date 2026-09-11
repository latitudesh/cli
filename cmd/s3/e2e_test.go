package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
)

// The binary-level tests below compile lsh once per test process and drive
// the real cobra wiring through exec. They are skipped with -short.
var (
	lshBinOnce sync.Once
	lshBinDir  string
	lshBinPath string
	lshBinErr  error
)

// TestMain removes the shared binary built by buildLshBinary once every test
// in the package has run (t.TempDir would delete it after the first test).
func TestMain(m *testing.M) {
	code := m.Run()
	if lshBinDir != "" {
		_ = os.RemoveAll(lshBinDir)
	}
	os.Exit(code)
}

// buildLshBinary builds the lsh binary once and returns its path. Every
// binary-level test shares the same build.
func buildLshBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the binary; skipped with -short")
	}
	lshBinOnce.Do(func() {
		root, err := filepath.Abs("../..")
		if err != nil {
			lshBinErr = err
			return
		}
		dir, err := os.MkdirTemp("", "lsh-e2e-")
		if err != nil {
			lshBinErr = err
			return
		}
		lshBinDir = dir
		lshBinPath = filepath.Join(dir, "lsh-e2e")
		build := exec.Command("go", "build", "-o", lshBinPath, ".")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			lshBinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if lshBinErr != nil {
		t.Fatal(lshBinErr)
	}
	return lshBinPath
}

// runResult is the outcome of one binary invocation.
type runResult struct {
	stdout, stderr string
	code           int
}

// runLsh executes the binary with env and args in dir, returning stdout,
// stderr and the exit code. Any failure that is not a non-zero exit is fatal.
func runLsh(t *testing.T, bin, dir string, env []string, args ...string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	res := runResult{stdout: o.String(), stderr: e.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v\nstdout=%q\nstderr=%q", args, err, res.stdout, res.stderr)
	}
	return res
}

// TestBinaryAgainstFakeS3 builds the real lsh binary and drives the cobra
// wiring end to end in the API-less mode (LSH_S3_ENDPOINT_URL + env
// credentials), covering flag parsing, output formats, dry-run, confirmation
// gating and exit codes that the unit tests bypass.
func TestBinaryAgainstFakeS3(t *testing.T) {
	bin := buildLshBinary(t)

	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	srv.AddObject("backups-7f3a", "2026/09/a.sql", []byte("aaa"), "application/sql")
	srv.AddObject("backups-7f3a", "2026/09/b.sql", []byte("bbbb"), "application/sql")
	srv.AddObject("backups-7f3a", "readme.txt", []byte("hi"), "text/plain")

	home := t.TempDir()
	work := t.TempDir()
	env := append(os.Environ(),
		"HOME="+home,
		"LSH_S3_ENDPOINT_URL="+srv.URL(),
		"LSH_S3_ACCESS_KEY_ID=AKIAEXAMPLE",
		"LSH_S3_SECRET_ACCESS_KEY=topsecret",
		"LATITUDESH_TOKEN=",
		"LSH_S3_USE_AWS_ENV=",
		"LSH_OUTPUT=",
		"LSH_CLASSIC_OUTPUT=true",
		"NO_COLOR=1",
	)

	run := func(args ...string) (stdout, stderr string, code int) {
		r := runLsh(t, bin, work, env, args...)
		return r.stdout, r.stderr, r.code
	}

	// ls default: the shared lsh table (headers + borders), like every other
	// list command; the prefix shows as a PRE row and the object by key.
	out, errOut, code := run("s3", "ls", "s3://backups-7f3a/")
	if code != 0 {
		t.Fatalf("ls exit %d: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "KEY") || !strings.Contains(out, "2026/") || !strings.Contains(out, "readme.txt") {
		t.Errorf("ls table output unexpected:\n%s", out)
	}
	if strings.Contains(out, "topsecret") || strings.Contains(errOut, "topsecret") {
		t.Fatal("secret leaked")
	}

	// ls -o json --query works (aws s3 ls ignores --output).
	out, errOut, code = run("s3", "ls", "s3://backups-7f3a/2026/09/", "-o", "json", "--query", "[].key")
	if code != 0 {
		t.Fatalf("ls json exit %d: %s%s", code, out, errOut)
	}
	var keys []string
	if err := json.Unmarshal([]byte(out), &keys); err != nil || len(keys) != 2 {
		t.Errorf("ls -o json --query: %v %q", err, out)
	}

	// cp upload with trailing slash keeps the file name; cp download to a
	// directory; cp to stdout prints only the payload.
	src := filepath.Join(work, "dump.sql")
	if err := os.WriteFile(src, []byte("select 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errOut, code = run("s3", "cp", src, "s3://backups-7f3a/2026/10/")
	if code != 0 || !strings.HasPrefix(out, "upload: ") || !strings.Contains(out, "to s3://backups-7f3a/2026/10/dump.sql") {
		t.Fatalf("cp upload: exit %d out=%q err=%q", code, out, errOut)
	}
	if o := srv.Object("backups-7f3a", "2026/10/dump.sql"); o == nil || string(o.Data) != "select 1;" || o.ContentType == "" {
		t.Fatalf("uploaded object missing or wrong: %+v", o)
	}
	if srv.HasChecksumHeaders() {
		t.Error("upload sent checksum headers")
	}
	dest := filepath.Join(work, "restore")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	out, errOut, code = run("s3", "cp", "s3://backups-7f3a/2026/10/dump.sql", dest+string(os.PathSeparator))
	if code != 0 || !strings.HasPrefix(out, "download: ") {
		t.Fatalf("cp download: exit %d out=%q err=%q", code, out, errOut)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "dump.sql")); err != nil || string(data) != "select 1;" {
		t.Fatalf("downloaded file: %v %q", err, data)
	}
	out, _, code = run("s3", "cp", "s3://backups-7f3a/readme.txt", "-")
	if code != 0 || out != "hi" {
		t.Errorf("cp to stdout: exit %d out=%q", code, out)
	}

	// stat object.
	out, errOut, code = run("s3", "stat", "s3://backups-7f3a/readme.txt", "-o", "json")
	if code != 0 || !strings.Contains(out, `"key": "readme.txt"`) {
		t.Errorf("stat: exit %d out=%q err=%q", code, out, errOut)
	}

	// presign prints only a URL.
	out, _, code = run("s3", "presign", "s3://backups-7f3a/readme.txt", "--expires-in", "15m")
	if code != 0 || !strings.HasPrefix(strings.TrimSpace(out), srv.URL()+"/backups-7f3a/readme.txt?") || strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("presign: exit %d out=%q", code, out)
	}
	if _, _, code = run("s3", "presign", "s3://backups-7f3a/readme.txt", "--expires-in", "8d"); code != 2 {
		t.Errorf("presign above 7d must be a usage error (2), got %d", code)
	}

	// rm --recursive --dryrun: plan lines, zero writes.
	srv.ResetRequests()
	out, errOut, code = run("s3", "rm", "s3://backups-7f3a/2026/09/", "--recursive", "--dryrun")
	if code != 0 || !strings.Contains(out, "(dryrun) delete: s3://backups-7f3a/2026/09/a.sql") {
		t.Errorf("rm dryrun: exit %d out=%q err=%q", code, out, errOut)
	}
	if len(srv.WriteRequests()) != 0 {
		t.Error("dry-run issued write requests")
	}

	// rm --recursive without --yes in a non-interactive session is refused (7).
	if _, errOut, code = run("s3", "rm", "s3://backups-7f3a/2026/09/", "--recursive"); code != 7 || !strings.Contains(errOut, "--yes") {
		t.Errorf("rm recursive non-interactive: exit %d err=%q", code, errOut)
	}
	// Whole bucket requires --all.
	if _, _, code = run("s3", "rm", "s3://backups-7f3a/", "--recursive", "--yes"); code != 2 {
		t.Errorf("rm whole bucket without --all must exit 2, got %d", code)
	}
	// With --yes it deletes.
	out, _, code = run("s3", "rm", "s3://backups-7f3a/2026/09/", "--recursive", "--yes")
	if code != 0 || strings.Count(out, "delete: ") != 2 || srv.Object("backups-7f3a", "2026/09/a.sql") != nil {
		t.Errorf("rm recursive: exit %d out=%q", code, out)
	}
	// Single delete of a missing key is idempotent.
	if _, _, code = run("s3", "rm", "s3://backups-7f3a/does-not-exist"); code != 0 {
		t.Errorf("rm missing key must exit 0, got %d", code)
	}

	// Unknown bucket → 3 (not found).
	if _, errOut, code = run("s3", "ls", "s3://nope-1234/"); code != 3 {
		t.Errorf("ls unknown bucket: exit %d err=%q", code, errOut)
	}

	// Unsupported aws flag gets an explanation, exit 2.
	if _, errOut, code = run("s3", "cp", src, "s3://backups-7f3a/x", "--acl", "public-read"); code != 2 || !strings.Contains(errOut, "--acl") {
		t.Errorf("--acl: exit %d err=%q", code, errOut)
	}

	// rb needs the API: refused in endpoint mode with a usage error.
	if _, _, code = run("s3", "rb", "s3://backups-7f3a"); code == 0 {
		t.Error("rb in endpoint-override mode must fail")
	}

	// mb is an API command: in endpoint-override mode it refuses with a usage
	// error; without the override, --dryrun prints the plan even when not
	// logged in.
	if _, _, code = run("s3", "mb", "s3://newbucket", "--region", "DAL", "--project", "proj_1", "--dryrun"); code != 2 {
		t.Errorf("mb with LSH_S3_ENDPOINT_URL set must exit 2, got %d", code)
	}
	{
		noOverride := append(append([]string{}, env...), "LSH_S3_ENDPOINT_URL=")
		r := runLsh(t, bin, work, noOverride, "s3", "mb", "s3://newbucket", "--region", "DAL", "--project", "proj_1", "--dryrun")
		if r.code != 0 || !strings.Contains(r.stdout, "(dryrun) make_bucket: s3://newbucket") {
			t.Errorf("mb dryrun: exit %d out=%q stderr=%q", r.code, r.stdout, r.stderr)
		}
	}

	// Missing credentials → 4 with the variable names in the message.
	noCreds := append(append([]string{}, env...), "LSH_S3_ACCESS_KEY_ID=", "LSH_S3_SECRET_ACCESS_KEY=")
	r := runLsh(t, bin, work, noCreds, "s3", "ls", "s3://backups-7f3a/")
	if r.code != 4 || !strings.Contains(r.stderr, "LSH_S3_ACCESS_KEY_ID") {
		t.Errorf("no credentials: exit %d stderr=%q", r.code, r.stderr)
	}
}

// Fixed identifiers served by the fake Latitude API.
const (
	e2eSecret      = "S3CR3T-e2e-xyz"
	e2eAccessKeyID = "AKIAE2E"
	e2eBucketName  = "bkt-e2e-7f3a"
	e2eBucketID    = "bkt_e2e"
)

// fakeLatitudeAPI is a minimal JSON:API server for the /storage endpoints the
// access-keys and bucket resolution paths call. It accepts any Authorization
// and API-Version header without validation and records what it served.
type fakeLatitudeAPI struct {
	srv   *httptest.Server
	s3URL string

	mu       sync.Mutex
	requests []string // "METHOD /path"
	created  []string // names of the keys created through POST
}

func newFakeLatitudeAPI(s3URL string) *fakeLatitudeAPI {
	f := &fakeLatitudeAPI{s3URL: s3URL}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *fakeLatitudeAPI) Close() { f.srv.Close() }

// HostPort returns "127.0.0.1:<port>" for --hostname.
func (f *fakeLatitudeAPI) HostPort() string { return strings.TrimPrefix(f.srv.URL, "http://") }

func (f *fakeLatitudeAPI) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeLatitudeAPI) Created() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.created...)
}

func (f *fakeLatitudeAPI) bucket() map[string]any {
	return map[string]any{
		"id":   e2eBucketID,
		"type": "object_storages",
		"attributes": map[string]any{
			"name":          "e2e",
			"bucket_name":   e2eBucketName,
			"storage_class": "standard",
			"endpoint":      f.s3URL,
			"versioning":    false,
			"locking":       false,
			// The real API serializes this as "" for buckets without object
			// lock; the SDK model wants a number, so the CLI must normalize it.
			"retention_period": "",
			"source":           "default",
			"region": map[string]any{
				"city": "Dallas",
				"site": map[string]any{"slug": "DAL"},
			},
			"project": map[string]any{"id": "proj_1", "slug": "e2e-project", "name": "E2E"},
		},
	}
}

func (f *fakeLatitudeAPI) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	writeJSON := func(status int, v any) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && path == "/storage/buckets":
		writeJSON(200, map[string]any{"data": []any{f.bucket()}})
	case r.Method == http.MethodGet && path == "/storage/buckets/"+e2eBucketID:
		writeJSON(200, map[string]any{"data": f.bucket()})
	case r.Method == http.MethodPost && path == "/storage/access_keys":
		var req struct {
			Data struct {
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &req)
		name := req.Data.Attributes.Name
		f.mu.Lock()
		f.created = append(f.created, name)
		f.mu.Unlock()
		writeJSON(201, map[string]any{"data": map[string]any{
			"type": "access_keys",
			"attributes": map[string]any{
				"access_key": map[string]any{
					"access_key_id":     e2eAccessKeyID,
					"secret_access_key": e2eSecret,
					"name":              name,
					"username":          "e2e+x@latitude.sh",
					"status":            "Active",
				},
			},
		}})
	case r.Method == http.MethodGet && path == "/storage/access_keys":
		writeJSON(200, map[string]any{"data": map[string]any{
			"standard": []any{map[string]any{
				"name":          "k1",
				"username":      "e2e+x@latitude.sh",
				"access_key_id": e2eAccessKeyID,
				"status":        "Active",
				"created_at":    "2026-09-07T10:00:00Z",
				"region":        "DAL",
				"access":        "rw",
				"buckets":       []string{e2eBucketName},
			}},
			"high_performance": []any{},
		}})
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/storage/access_keys/"):
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(404, map[string]any{"errors": []any{map[string]any{
			"status": "404", "title": "Not Found", "detail": r.Method + " " + r.URL.Path,
		}}})
	}
}

// writeE2EConfig writes ~/.config/lsh/config.json under home with a profile
// named e2e as the default. jsonDefault adds the top-level "json": true that
// the config file uses to make json the default output format.
func writeE2EConfig(t *testing.T, home string, jsonDefault bool) string {
	t.Helper()
	cfg := map[string]any{
		"default_profile": "e2e",
		"profiles": map[string]any{
			"e2e": map[string]any{"authorization": "test-token"},
		},
	}
	if jsonDefault {
		cfg["json"] = true
	}
	dir := filepath.Join(home, ".config", "lsh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// e2eEnv is the environment for the API-backed runs: token from the
// environment, no S3 endpoint override, no env credentials, and no inherited
// output preference unless the case sets one.
func e2eEnv(home string, extra ...string) []string {
	env := append(os.Environ(),
		"HOME="+home,
		"LATITUDESH_TOKEN=test-token",
		"LSH_S3_ENDPOINT_URL=",
		"LSH_S3_ACCESS_KEY_ID=",
		"LSH_S3_SECRET_ACCESS_KEY=",
		"LSH_S3_USE_AWS_ENV=",
		"LSH_OUTPUT=",
		"LSH_PROFILE=",
		"LSH_PROJECT=",
		"LSH_CLASSIC_OUTPUT=true",
		"NO_COLOR=1",
	)
	return append(env, extra...)
}

// jsonField decodes structured output (one object, or a one-element array)
// and returns the string value of field.
func jsonField(t *testing.T, out, field string) (string, bool) {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	obj, ok := v.(map[string]any)
	if arr, isArr := v.([]any); isArr {
		if len(arr) != 1 {
			t.Fatalf("expected one JSON object, got %d: %s", len(arr), out)
		}
		obj, ok = arr[0].(map[string]any)
	}
	if !ok {
		t.Fatalf("expected a JSON object: %s", out)
	}
	val, present := obj[field]
	if !present {
		return "", false
	}
	s, _ := val.(string)
	return s, true
}

// TestBinaryNoSecretContract drives the binary against a fake Latitude API
// and the fake S3 endpoint to pin the secret-display contract of
// `access-keys create` (spec section 10) and the cobra error handling of
// newCmd (one message, exit 2). Unless a case says otherwise, neither stdout
// nor stderr may contain the secret.
func TestBinaryNoSecretContract(t *testing.T) {
	bin := buildLshBinary(t)

	s3srv := s3test.New()
	defer s3srv.Close()
	s3srv.CreateBucket(e2eBucketName)
	s3srv.AddObject(e2eBucketName, "readme.txt", []byte("hi"), "text/plain")

	api := newFakeLatitudeAPI(s3srv.URL())
	defer api.Close()

	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	apiArgs := []string{"--hostname", api.HostPort(), "--scheme", "http"}
	run := func(env []string, args ...string) runResult {
		t.Helper()
		return runLsh(t, bin, work, env, append(append([]string{}, args...), apiArgs...)...)
	}
	noSecret := func(name string, r runResult) {
		t.Helper()
		if strings.Contains(r.stdout, e2eSecret) {
			t.Errorf("%s: secret leaked on stdout:\n%s", name, r.stdout)
		}
		if strings.Contains(r.stderr, e2eSecret) {
			t.Errorf("%s: secret leaked on stderr:\n%s", name, r.stderr)
		}
	}
	createArgs := []string{"s3", "access-keys", "create", "--bucket", "e2e", "--name", "k1"}

	home := t.TempDir()
	writeE2EConfig(t, home, false)
	env := e2eEnv(home)

	// 8. Cobra errors: printed once, exit 2, no usage dump. These never reach
	// the API, so they run first and report even when the API cases cannot.
	r := run(env, "s3", "ls", "--bogus-flag")
	if r.code != 2 || strings.Count(r.stderr, "unknown flag") != 1 {
		t.Errorf("ls --bogus-flag: exit %d (want 2), stderr must mention 'unknown flag' once:\n%s", r.code, r.stderr)
	}
	if strings.Contains(r.stderr, "Usage:") {
		t.Errorf("ls --bogus-flag: usage must not be echoed:\n%s", r.stderr)
	}
	r = run(env, "s3", "stat")
	if r.code != 2 || strings.TrimSpace(r.stderr) == "" || strings.Count(r.stderr, "accepts 1 arg") != 1 {
		t.Errorf("stat without args: exit %d (want 2), stderr=%q", r.code, r.stderr)
	}
	r = run(env, "s3", "cp", "onlyone")
	if r.code != 2 || strings.TrimSpace(r.stderr) == "" {
		t.Errorf("cp with one arg: exit %d (want 2), stderr=%q", r.code, r.stderr)
	}

	// 9. aws-style --region on an object command is redirected to
	// --signing-region with exit 2 (checked in PreRunE, before any API call).
	r = run(env, "s3", "cp", "./f", "s3://e2e/", "--region", "us-east-1")
	if r.code != 2 || !strings.Contains(r.stderr, "--signing-region") {
		t.Errorf("cp --region: exit %d (want 2), stderr must explain --signing-region:\n%s", r.code, r.stderr)
	}
	if strings.Contains(r.stderr, "unknown flag") {
		t.Errorf("cp --region must not be reported as an unknown flag:\n%s", r.stderr)
	}

	// 1. Human output, not saved: the secret is shown in clear, exactly once,
	// on stdout (it cannot be retrieved again), never on stderr.
	r = run(env, createArgs...)
	if r.code != 0 {
		reqs := api.Requests()
		hint := ""
		if len(reqs) == 0 {
			hint = "\nthe fake API received no request: the SDK client ignores --hostname/--scheme (objectstorage.NewAPIClient builds sdk.New without WithServerURL)"
		}
		t.Fatalf("create (human): exit %d\nstdout=%s\nstderr=%s\napi requests=%v%s", r.code, r.stdout, r.stderr, reqs, hint)
	}
	if n := strings.Count(r.stdout, e2eSecret); n != 1 {
		t.Errorf("create (human): secret must be printed exactly once on stdout, found %d:\n%s", n, r.stdout)
	}
	if strings.Contains(r.stderr, e2eSecret) {
		t.Errorf("create (human): secret on stderr:\n%s", r.stderr)
	}
	if !strings.Contains(r.stdout, e2eAccessKeyID) {
		t.Errorf("create (human): access key id missing:\n%s", r.stdout)
	}
	if got := api.Created(); len(got) != 1 || got[0] != "k1" {
		t.Errorf("fake API saw created keys %v, want [k1]", got)
	}

	// 2. LSH_OUTPUT=json without -o: structured output does not embed the
	// secret and stderr tells how to get it.
	r = run(e2eEnv(home, "LSH_OUTPUT=json"), createArgs...)
	if r.code != 0 {
		t.Fatalf("create (LSH_OUTPUT=json): exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	noSecret("create (LSH_OUTPUT=json)", r)
	if _, present := jsonField(t, r.stdout, "secret_access_key"); present {
		t.Errorf("create (LSH_OUTPUT=json): secret_access_key present in JSON:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "--show-secret") || !strings.Contains(r.stderr, "-o json") {
		t.Errorf("create (LSH_OUTPUT=json): stderr must point at --show-secret / -o json:\n%s", r.stderr)
	}

	// 3. Explicit -o json embeds the secret.
	r = run(env, append(createArgs, "-o", "json")...)
	if r.code != 0 {
		t.Fatalf("create (-o json): exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	if got, _ := jsonField(t, r.stdout, "secret_access_key"); got != e2eSecret {
		t.Errorf("create (-o json): secret_access_key=%q, want %q:\n%s", got, e2eSecret, r.stdout)
	}
	if strings.Contains(r.stderr, e2eSecret) {
		t.Errorf("create (-o json): secret on stderr:\n%s", r.stderr)
	}

	// 4. --json on the command line counts as explicit too.
	r = run(env, append(createArgs, "--json")...)
	if r.code != 0 {
		t.Fatalf("create (--json): exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	if got, _ := jsonField(t, r.stdout, "secret_access_key"); got != e2eSecret {
		t.Errorf("create (--json): secret_access_key=%q, want %q:\n%s", got, e2eSecret, r.stdout)
	}

	// 5. "json": true in the config file is not an explicit request.
	{
		jsonHome := t.TempDir()
		writeE2EConfig(t, jsonHome, true)
		r = run(e2eEnv(jsonHome), createArgs...)
		if r.code != 0 {
			t.Fatalf("create (config json=true): exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
		}
		noSecret("create (config json=true)", r)
		if _, present := jsonField(t, r.stdout, "secret_access_key"); present {
			t.Errorf("create (config json=true): secret_access_key present in JSON:\n%s", r.stdout)
		}
		if !strings.Contains(r.stderr, "--show-secret") {
			t.Errorf("create (config json=true): stderr must point at --show-secret:\n%s", r.stderr)
		}
	}

	// 6. --save stores the key in profile e2e and prints no secret anywhere.
	r = run(env, append(createArgs, "--save")...)
	if r.code != 0 {
		t.Fatalf("create --save: exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	noSecret("create --save", r)
	if !strings.Contains(r.stdout+r.stderr, "Saved as") {
		t.Errorf("create --save: expected a 'Saved as' confirmation:\nstdout=%s\nstderr=%s", r.stdout, r.stderr)
	}
	{
		data, err := os.ReadFile(filepath.Join(home, ".config", "lsh", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Profiles map[string]struct {
				ObjectStorage struct {
					Keys map[string]struct {
						AccessKeyID     string `json:"access_key_id"`
						SecretAccessKey string `json:"secret_access_key"`
					} `json:"keys"`
				} `json:"object_storage"`
			} `json:"profiles"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("config.json: %v\n%s", err, data)
		}
		found := false
		for name, k := range cfg.Profiles["e2e"].ObjectStorage.Keys {
			if k.AccessKeyID == e2eAccessKeyID {
				found = true
				if k.SecretAccessKey != e2eSecret {
					t.Errorf("saved key %q has secret %q, want the API secret", name, k.SecretAccessKey)
				}
			}
		}
		if !found {
			t.Errorf("config.json has no key with access_key_id %s under profile e2e:\n%s", e2eAccessKeyID, data)
		}
	}

	// 6b. --save with an unwritable profile: the key cannot be stored, so it is
	// deleted again. Nothing is left live on the API and the one-time secret is
	// never printed on either stream.
	if os.Geteuid() != 0 {
		roHome := t.TempDir()
		cfgDir := filepath.Join(roHome, ".config", "lsh")
		writeE2EConfig(t, roHome, false)
		if err := os.Chmod(cfgDir, 0o500); err != nil {
			t.Fatal(err)
		}
		// Restore write access so the temp dir can be cleaned up.
		t.Cleanup(func() { _ = os.Chmod(cfgDir, 0o700) })

		r = run(e2eEnv(roHome), append(createArgs, "--save", "-o", "json")...)
		// --save means create *and* persist: a caller must not read exit 0 as
		// "the credential is in the profile".
		if r.code != 1 {
			t.Fatalf("create --save (unwritable profile): exit %d (want 1)\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, "not saved in the profile") {
			t.Errorf("create --save (unwritable profile): stderr must state the outcome:\n%s", r.stderr)
		}
		// The key does not exist any more, so there is no document to emit:
		// stdout stays empty rather than describing a credential that is gone.
		if strings.TrimSpace(r.stdout) != "" {
			t.Errorf("create --save (unwritable profile): stdout must stay empty, got:\n%s", r.stdout)
		}
		if strings.Contains(r.stderr, e2eSecret) {
			t.Errorf("create --save (unwritable profile): the key was removed, so no secret may be printed:\n%s", r.stderr)
		}
		for _, want := range []string{"could not be saved", "It was deleted again", "Retry once the profile is writable"} {
			if !strings.Contains(r.stderr, want) {
				t.Errorf("create --save (unwritable profile): stderr lacks %q:\n%s", want, r.stderr)
			}
		}
		deletes := 0
		for _, req := range api.Requests() {
			if strings.HasPrefix(req, "DELETE ") && strings.Contains(req, "/storage/access_keys") {
				deletes++
			}
		}
		if deletes != 1 {
			t.Errorf("create --save (unwritable profile): expected one access-key delete, got %d (%v)", deletes, api.Requests())
		}
		if strings.Contains(r.stderr, "it was saved in the profile") {
			t.Errorf("create --save (unwritable profile): stderr must not claim the key was saved:\n%s", r.stderr)
		}
	}

	// 7. The saved key is selected automatically for the bucket; --debug
	// tracing never shows the secret and the listing reaches the S3 server.
	r = run(env, "s3", "ls", "s3://e2e/", "--debug")
	if r.code != 0 {
		t.Fatalf("ls --debug with saved key: exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	noSecret("ls --debug", r)
	if !strings.Contains(r.stdout, " readme.txt") {
		t.Errorf("ls --debug: listing missing the seeded object:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "saved key") {
		t.Errorf("ls --debug: expected the credential source in the debug trace:\n%s", r.stderr)
	}

	// The list command exposes the id and scope but never a secret.
	r = run(env, "s3", "access-keys", "list", "--project", "e2e-project", "-o", "json")
	if r.code != 0 {
		t.Fatalf("access-keys list: exit %d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	noSecret("access-keys list", r)
	if !strings.Contains(r.stdout, e2eAccessKeyID) {
		t.Errorf("access-keys list: access key id missing:\n%s", r.stdout)
	}
}

// TestBinaryFlagValidationExitCodes covers the exit-code contract for
// command-line errors detected by the root pre-run: an invalid --output or
// --query is a usage error (2), not a generic failure (1).
func TestBinaryFlagValidationExitCodes(t *testing.T) {
	bin := buildLshBinary(t)
	home := t.TempDir()
	writeE2EConfig(t, home, false)
	env := e2eEnv(home)
	work := t.TempDir()

	cases := []struct {
		name string
		args []string
	}{
		{"invalid output format", []string{"s3", "ls", "-o", "bogus"}},
		{"invalid query", []string{"s3", "ls", "--query", "[[["}},
		{"invalid page size", []string{"s3", "ls", "--page-size", "-1"}},
	}
	for _, c := range cases {
		r := runLsh(t, bin, work, env, c.args...)
		if r.code != 2 {
			t.Errorf("%s: exit %d, want 2\nstderr=%s", c.name, r.code, r.stderr)
		}
		if strings.TrimSpace(r.stderr) == "" {
			t.Errorf("%s: expected an actionable message on stderr", c.name)
		}
	}
}

// TestBinaryLegacyExitCodesUnchanged pins the boundary of the documented S3
// exit codes: they apply to the s3 subtree (including its legacy aliases) and
// must not change what the older command groups return.
func TestBinaryLegacyExitCodesUnchanged(t *testing.T) {
	bin := buildLshBinary(t)
	home := t.TempDir()
	writeE2EConfig(t, home, false)
	env := e2eEnv(home)
	work := t.TempDir()

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"s3 opted in", []string{"s3", "ls", "-o", "bogus"}, 2},
		{"s3 through the legacy alias", []string{"storage-objects", "list", "-o", "bogus"}, 2},
		{"older group keeps exiting 1", []string{"servers", "list", "-o", "bogus"}, 1},
		{"older group, bad page size", []string{"servers", "list", "--page-size", "-1"}, 1},
	}
	for _, c := range cases {
		r := runLsh(t, bin, work, env, c.args...)
		if r.code != c.want {
			t.Errorf("%s: exit %d, want %d\nstderr=%s", c.name, r.code, c.want, r.stderr)
		}
	}
}
