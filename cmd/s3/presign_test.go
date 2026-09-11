package s3

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/objectstorage/s3test"
)

func presignTestSession(t *testing.T, srv *s3test.Server, region string) *Session {
	t.Helper()
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "b", BucketName: "backups-7f3a", Endpoint: srv.URL(), StorageClass: "standard", SigningRegion: region}
	cred := objectstorage.NewCredential("AKIAEXAMPLE", "topsecret", "test")
	client, err := objectstorage.NewS3Client(b, cred, objectstorage.ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	return &Session{Bucket: b, Cred: cred, Client: client}
}

func TestParseExpiresIn(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr string
	}{
		{in: "3600", want: time.Hour},
		{in: "1", want: time.Second},
		{in: "604800", want: 7 * 24 * time.Hour},
		{in: "15m", want: 15 * time.Minute},
		{in: "90s", want: 90 * time.Second},
		{in: "1h", want: time.Hour},
		{in: "7d", want: 7 * 24 * time.Hour},
		{in: "1h30m", want: 90 * time.Minute},
		{in: " 2h ", want: 2 * time.Hour},
		{in: "8d", wantErr: "exceeds the 7 day maximum"},
		{in: "604801", wantErr: "exceeds the 7 day maximum"},
		{in: "0", wantErr: "at least 1 second"},
		{in: "junk", wantErr: "invalid --expires-in"},
		{in: "10x", wantErr: "invalid --expires-in"},
		{in: "-5", wantErr: "invalid --expires-in"},
		{in: "", wantErr: "cannot be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseExpiresIn(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				if exitcode.Of(err) != exitcode.Usage {
					t.Errorf("exit code = %d, want %d", exitcode.Of(err), exitcode.Usage)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPresignParseOptions(t *testing.T) {
	if o, err := presignParseOptions("get", "3600", "", ""); err != nil || o.Method != http.MethodGet || o.Expires != time.Hour {
		t.Errorf("lower-case get: %+v %v", o, err)
	}
	if o, err := presignParseOptions("", "60", "", ""); err != nil || o.Method != http.MethodGet {
		t.Errorf("empty method defaults to GET: %+v %v", o, err)
	}
	if o, err := presignParseOptions("head", "60", "v1", ""); err != nil || o.Method != http.MethodHead || o.VersionID != "v1" {
		t.Errorf("head with version: %+v %v", o, err)
	}
	if _, err := presignParseOptions("POST", "60", "", ""); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("POST must be a usage error, got %v", err)
	}
	if _, err := presignParseOptions("PUT", "60", "v1", ""); exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "--version-id") {
		t.Errorf("PUT with version-id must be a usage error, got %v", err)
	}
	if _, err := presignParseOptions("GET", "60", "", "text/plain"); exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "--content-type") {
		t.Errorf("GET with content-type must be a usage error, got %v", err)
	}
	if o, err := presignParseOptions("put", "60", "", "text/plain"); err != nil || o.ContentType != "text/plain" {
		t.Errorf("PUT with content-type: %+v %v", o, err)
	}
	if _, err := presignParseOptions("GET", "8d", "", ""); exitcode.Of(err) != exitcode.Usage {
		t.Errorf("8d must be a usage error, got %v", err)
	}
}

func TestPresignGetURL(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	sess := presignTestSession(t, srv, "eu-west-2")

	before := time.Now()
	res, err := presignObject(context.Background(), sess, "2026/09/report.pdf", presignOptions{Method: http.MethodGet, Expires: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != http.MethodGet {
		t.Errorf("method = %q", res.Method)
	}
	if res.ExpiresAt.Before(before.Add(15*time.Minute-time.Second)) || res.ExpiresAt.After(time.Now().Add(15*time.Minute+time.Second)) {
		t.Errorf("expires_at = %v", res.ExpiresAt)
	}
	u, err := url.Parse(res.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.URL, srv.URL()) {
		t.Errorf("url %q not on endpoint %s", res.URL, srv.URL())
	}
	if u.Path != "/backups-7f3a/2026/09/report.pdf" {
		t.Errorf("path = %q, want path-style /<bucket>/<key>", u.Path)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "900" {
		t.Errorf("X-Amz-Expires = %q", q.Get("X-Amz-Expires"))
	}
	if cred := q.Get("X-Amz-Credential"); !strings.Contains(cred, "/eu-west-2/s3/aws4_request") || !strings.HasPrefix(cred, "AKIAEXAMPLE/") {
		t.Errorf("X-Amz-Credential = %q", cred)
	}
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || q.Get("X-Amz-Signature") == "" {
		t.Errorf("missing SigV4 query parameters: %v", q)
	}
	if q.Has("versionId") {
		t.Error("versionId must be absent when not requested")
	}
	if strings.Contains(res.URL, "topsecret") {
		t.Error("URL leaked the secret")
	}
	// Presigning is local: the bucket is never contacted.
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent while presigning, want 0", n)
	}

	var out bytes.Buffer
	presignWrite(&out, res)
	if out.String() != res.URL+"\n" {
		t.Errorf("human output = %q", out.String())
	}
}

func TestPresignVersionIDAndHead(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	sess := presignTestSession(t, srv, "us-east-1")

	res, err := presignObject(context.Background(), sess, "k", presignOptions{Method: http.MethodHead, Expires: time.Hour, VersionID: "3HL4kqtJlcpXroDTDmJ"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(res.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("versionId") != "3HL4kqtJlcpXroDTDmJ" {
		t.Errorf("versionId missing: %s", res.URL)
	}
	if u.Query().Get("X-Amz-Expires") != "3600" {
		t.Errorf("X-Amz-Expires = %q", u.Query().Get("X-Amz-Expires"))
	}
	if res.Method != http.MethodHead {
		t.Errorf("method = %q", res.Method)
	}
}

func TestPresignPutURL(t *testing.T) {
	srv := s3test.New()
	defer srv.Close()
	srv.CreateBucket("backups-7f3a")
	sess := presignTestSession(t, srv, "us-east-1")

	res, err := presignObject(context.Background(), sess, "in/upload.bin", presignOptions{Method: http.MethodPut, Expires: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != http.MethodPut {
		t.Errorf("method = %q", res.Method)
	}
	u, err := url.Parse(res.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/backups-7f3a/in/upload.bin" || u.Query().Get("X-Amz-Expires") != "604800" {
		t.Errorf("url = %s", res.URL)
	}
	if len(srv.Requests()) != 0 {
		t.Error("presigning must not contact the backend")
	}

	// The URL actually works against the fake server without credentials.
	req, err := http.NewRequest(http.MethodPut, res.URL, strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("PUT with presigned URL = %d", resp.StatusCode)
	}
	if o := srv.Object("backups-7f3a", "in/upload.bin"); o == nil || string(o.Data) != "payload" {
		t.Errorf("uploaded object = %+v", o)
	}
}

// TestPresignCommandRejectsRegionFlag guards the aws-compat shim on presign:
// --region must fail with exit 2 and point to --signing-region, before the
// bucket is looked up.
func TestPresignCommandRejectsRegionFlag(t *testing.T) {
	cmd := NewPresignCmd()
	Finalize(cmd) // production installs this in build_s3.go
	cmd.SetArgs([]string{"s3://backups/report.pdf", "--region", "us-east-1"})
	var err error
	_, stderr := rmCaptureOutput(t, func() { err = cmd.Execute() })
	if exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("exit code = %d, want %d (%v)", exitcode.Of(err), exitcode.Usage, err)
	}
	if !strings.Contains(stderr, "--region is not supported") || !strings.Contains(stderr, "--signing-region") {
		t.Errorf("stderr lacks the directed --region explanation:\n%s", stderr)
	}
}

// TestPresignRejectsPrefixWithoutRecursiveHint covers the message fix: the
// shared object parser points at --recursive, a flag presign does not have.
func TestPresignRejectsPrefixWithoutRecursiveHint(t *testing.T) {
	cmd := NewPresignCmd()
	Finalize(cmd)
	cmd.SetArgs([]string{"s3://backups/2026/"})
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a usage error for a prefix")
	}
	if code := exitcode.Of(err); code != exitcode.Usage {
		t.Errorf("exit code = %d, want %d", code, exitcode.Usage)
	}
	if strings.Contains(err.Error(), "--recursive") {
		t.Errorf("presign has no --recursive; message was %q", err)
	}
	if !strings.Contains(err.Error(), "single object") {
		t.Errorf("message %q should say presign signs one object", err)
	}
}
