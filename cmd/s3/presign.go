package s3

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/spf13/cobra"
)

// presignMaxExpiry is the SigV4 ceiling for pre-signed URLs (7 days).
const presignMaxExpiry = 7 * 24 * time.Hour

// presignOptions are the parsed flags of `lsh s3 presign`.
type presignOptions struct {
	Method      string
	Expires     time.Duration
	VersionID   string
	ContentType string
}

// NewPresignCmd builds `lsh s3 presign s3://bucket/key`.
func NewPresignCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "presign s3://bucket/key",
		Aliases: []string{"presigned-url", "share"},
		Short:   "Generate a pre-signed URL",
		Long: `Generate a pre-signed URL that grants temporary access to one object
without sharing credentials.

The URL is signed locally with the access key the CLI selects for the bucket
(the only network call is the bucket lookup). --expires-in accepts seconds
(3600) or a duration with an s/m/h/d suffix (15m, 1h, 7d); the maximum is 7
days (604800 seconds). --method PUT signs an upload URL and requires a key
with write permission. In the default output only the URL is printed, so the
command composes with $(...) and curl.`,
		Example: `  lsh s3 presign s3://backups/report.pdf
  lsh s3 presign s3://backups/report.pdf --expires-in 15m
  curl -o report.pdf "$(lsh s3 presign s3://backups/report.pdf)"
  lsh s3 presign s3://uploads/in.bin --method PUT --expires-in 1h
  lsh s3 presign s3://backups/report.pdf -o json`,
		Args: cobra.ExactArgs(1),
	})
	f := cmd.Flags()
	f.String("expires-in", "3600", "how long the URL stays valid: seconds or a duration (15m, 1h, 7d); max 7d")
	f.String("method", "GET", "HTTP method to sign: GET, PUT or HEAD")
	f.String("version-id", "", "sign a URL for this specific version (GET/HEAD only)")
	f.String("content-type", "", "content type the uploader will send (PUT only; informational, see notes)")
	addProjectFlag(cmd, true, "project the bucket belongs to (disambiguates buckets with the same name)")
	addBucketFilterFlags(cmd)
	rejectRegionFlag(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		expiresIn, _ := f.GetString("expires-in")
		method, _ := f.GetString("method")
		versionID, _ := f.GetString("version-id")
		contentType, _ := f.GetString("content-type")

		// ObjectRef's shared message points at --recursive, a flag only rm and
		// the transfer commands have; presign signs exactly one object.
		if r, parseErr := objectstorage.ParseRemote(args[0]); parseErr == nil && strings.HasSuffix(r.Key, "/") {
			return printErr(objectstorage.ErrUsagef("key %q ends with '/'; presign signs a single object (use 'lsh s3 ls %s' to list the prefix)", r.Key, r))
		}
		ref, err := objectstorage.ObjectRef(args[0], false)
		if err != nil {
			return printErr(err)
		}
		opts, err := presignParseOptions(method, expiresIn, versionID, contentType)
		if err != nil {
			return printErr(err)
		}

		ctx, stop := objectstorage.SignalContext(context.Background())
		defer stop()

		sess, err := openBucket(ctx, cmd, ref.Bucket, opts.Method == http.MethodPut)
		if err != nil {
			return printErr(err)
		}
		res, err := presignObject(ctx, sess, ref.Key, opts)
		if err != nil {
			return printErr(err)
		}
		if isHuman() {
			presignWrite(os.Stdout, res)
			return nil
		}
		render([]renderer.ResponseData{res})
		return nil
	}
	return cmd
}

// presignParseOptions validates the flag values.
func presignParseOptions(method, expiresIn, versionID, contentType string) (presignOptions, error) {
	opts := presignOptions{VersionID: versionID, ContentType: contentType}
	opts.Method = strings.ToUpper(strings.TrimSpace(method))
	switch opts.Method {
	case http.MethodGet, http.MethodPut, http.MethodHead:
	case "":
		opts.Method = http.MethodGet
	default:
		return opts, objectstorage.ErrUsagef("--method must be GET, PUT or HEAD (got %q)", method)
	}
	d, err := parseExpiresIn(expiresIn)
	if err != nil {
		return opts, err
	}
	opts.Expires = d
	if opts.VersionID != "" && opts.Method == http.MethodPut {
		return opts, objectstorage.ErrUsagef("--version-id only applies to GET and HEAD URLs")
	}
	if opts.ContentType != "" && opts.Method != http.MethodPut {
		return opts, objectstorage.ErrUsagef("--content-type only applies to --method PUT")
	}
	return opts, nil
}

var expiresInRe = regexp.MustCompile(`^(\d+)\s*([smhd])$`)

// parseExpiresIn accepts a plain integer (seconds) or a duration with an
// s/m/h/d suffix (15m, 1h, 7d; composite Go durations such as 1h30m also
// work). Values above 7 days are a usage error rather than being clipped.
func parseExpiresIn(s string) (time.Duration, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return 0, objectstorage.ErrUsagef("--expires-in cannot be empty")
	}
	var d time.Duration
	switch {
	case isDigits(v):
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, objectstorage.ErrUsagef("invalid --expires-in %q: %v", s, err)
		}
		d = time.Duration(n) * time.Second
	default:
		if m := expiresInRe.FindStringSubmatch(v); m != nil {
			n, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				return 0, objectstorage.ErrUsagef("invalid --expires-in %q: %v", s, err)
			}
			unit := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[2]]
			d = time.Duration(n) * unit
		} else {
			parsed, err := time.ParseDuration(v)
			if err != nil {
				return 0, objectstorage.ErrUsagef("invalid --expires-in %q: use seconds (3600) or a duration such as 15m, 1h or 7d", s)
			}
			d = parsed
		}
	}
	if d < time.Second {
		return 0, objectstorage.ErrUsagef("--expires-in must be at least 1 second (got %q)", s)
	}
	if d > presignMaxExpiry {
		return 0, objectstorage.ErrUsagef("--expires-in %q exceeds the 7 day maximum (604800 seconds) for pre-signed URLs", s)
	}
	return d, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// presignObject signs the URL locally for the given method.
func presignObject(ctx context.Context, sess *Session, key string, opts presignOptions) (objectstorage.PresignResult, error) {
	if opts.ContentType != "" && opts.Method == http.MethodPut {
		objectstorage.Warnf("--content-type is not part of the signature; the uploader must send Content-Type: %s itself (a mismatch is accepted by most backends but not enforced)", opts.ContentType)
	}
	params := url.Values{}
	if opts.VersionID != "" {
		params.Set("versionId", opts.VersionID)
	}
	expiresAt := time.Now().Add(opts.Expires)
	var (
		u   *url.URL
		err error
	)
	switch opts.Method {
	case http.MethodPut:
		u, err = sess.Client.PresignedPutObject(ctx, sess.Bucket.BucketName, key, opts.Expires)
	case http.MethodHead:
		u, err = sess.Client.PresignedHeadObject(ctx, sess.Bucket.BucketName, key, opts.Expires, params)
	default:
		u, err = sess.Client.PresignedGetObject(ctx, sess.Bucket.BucketName, key, opts.Expires, params)
	}
	if err != nil {
		return objectstorage.PresignResult{}, sess.humanize(err)
	}
	if u == nil {
		return objectstorage.PresignResult{}, exitcode.Errorf(exitcode.Generic, "presigning returned no URL")
	}
	return objectstorage.PresignResult{URL: u.String(), Method: opts.Method, ExpiresAt: expiresAt}, nil
}

// presignWrite prints the human output: only the URL, so the command composes
// with $(...) and pipes.
func presignWrite(w io.Writer, res objectstorage.PresignResult) {
	fmt.Fprintln(w, res.URL)
}
