package objectstorage

import (
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
)

// Ref is a parsed command-line reference to a local path, stdin/stdout or an
// object storage location.
//
// Remote references follow the aws shape: `s3://<bucket>[/<key>]`. `lsh://` is
// accepted as a synonym. <bucket> may be the bucket's display name, its
// `bkt_…` ID or the backend bucket name; the resolver decides which.
type Ref struct {
	// Raw is the argument as typed.
	Raw string
	// Bucket is the bucket token (display name, bkt_ id or backend name).
	Bucket string
	// Key is the object key or prefix; empty when the reference is the bucket
	// itself.
	Key string
	// Remote is true for s3:// and lsh:// references (or bare bucket tokens
	// parsed through ParseRemote).
	Remote bool
	// Stdio is true when the argument is "-" (stdin for sources, stdout for
	// destinations).
	Stdio bool
	// HadScheme records whether an explicit scheme was present.
	HadScheme bool
}

const (
	schemeS3  = "s3://"
	schemeLsh = "lsh://"
)

// IsDir reports whether the reference denotes a prefix (empty key or a key
// ending in "/"), following the aws convention for trailing slashes.
func (r Ref) IsDir() bool {
	return r.Remote && (r.Key == "" || strings.HasSuffix(r.Key, "/"))
}

// String renders the canonical s3:// form for remote references and the raw
// argument otherwise.
func (r Ref) String() string {
	if !r.Remote {
		return r.Raw
	}
	if r.Key == "" {
		return schemeS3 + r.Bucket
	}
	return schemeS3 + r.Bucket + "/" + r.Key
}

// WithKey returns a copy of r pointing at key.
func (r Ref) WithKey(key string) Ref {
	r.Key = key
	return r
}

// stripScheme removes a leading s3:// or lsh:// and reports whether one was
// present.
func stripScheme(s string) (string, bool) {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, schemeS3):
		return s[len(schemeS3):], true
	case strings.HasPrefix(lower, schemeLsh):
		return s[len(schemeLsh):], true
	}
	return s, false
}

// ParseRemote parses an argument that must name a bucket or object. The
// scheme is optional (`ls`, `stat`, `rm`, `presign`, `lifecycle`, `metrics`
// accept `my-bucket/prefix/` as well as `s3://my-bucket/prefix/`).
func ParseRemote(arg string) (Ref, error) {
	s := strings.TrimSpace(arg)
	if s == "" {
		return Ref{}, exitcode.Errorf(exitcode.Usage, "missing bucket: expected s3://<bucket>[/<key>]")
	}
	if s == "-" {
		return Ref{}, exitcode.Errorf(exitcode.Usage, "%q is not a valid bucket reference", arg)
	}
	rest, had := stripScheme(s)
	if strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, ".") {
		return Ref{}, exitcode.Errorf(exitcode.Usage, "%q looks like a local path; expected s3://<bucket>[/<key>]", arg)
	}
	bucket, key := rest, ""
	if i := strings.Index(rest, "/"); i >= 0 {
		bucket, key = rest[:i], rest[i+1:]
	}
	if bucket == "" {
		return Ref{}, exitcode.Errorf(exitcode.Usage, "%q has no bucket name; expected s3://<bucket>[/<key>]", arg)
	}
	return Ref{Raw: arg, Bucket: bucket, Key: key, Remote: true, HadScheme: had}, nil
}

// ParseBucketOnly parses an argument that must name a bucket without a key
// (`mb`, `rb`). It mirrors the aws error for `aws s3 rb s3://b/key`.
func ParseBucketOnly(arg string) (Ref, error) {
	r, err := ParseRemote(arg)
	if err != nil {
		return r, err
	}
	if r.Key != "" {
		return r, exitcode.Errorf(exitcode.Usage, "please specify a valid bucket name only (got %q)", arg)
	}
	return r, nil
}

// ParseTransferArg parses a `cp`/`mv`/`sync` operand. Only an explicit
// scheme makes it remote; "-" is stdin/stdout; anything else is a local path.
// This is the aws rule that keeps `cp ./file s3://b/` unambiguous.
func ParseTransferArg(arg string) (Ref, error) {
	s := strings.TrimSpace(arg)
	if s == "" {
		return Ref{}, exitcode.Errorf(exitcode.Usage, "empty path argument")
	}
	if s == "-" {
		return Ref{Raw: arg, Stdio: true}, nil
	}
	if _, had := stripScheme(s); had {
		return ParseRemote(s)
	}
	return Ref{Raw: arg}, nil
}

// ObjectRef parses an argument that must name an object (bucket and non-empty
// key). Keys ending in "/" are rejected unless allowPrefix is true.
func ObjectRef(arg string, allowPrefix bool) (Ref, error) {
	r, err := ParseRemote(arg)
	if err != nil {
		return r, err
	}
	if r.Key == "" {
		return r, exitcode.Errorf(exitcode.Usage, "%s names a bucket, not an object; expected s3://<bucket>/<key>", r)
	}
	if !allowPrefix && strings.HasSuffix(r.Key, "/") {
		return r, exitcode.Errorf(exitcode.Usage, "key %q ends with '/'; use --recursive to operate on a prefix", r.Key)
	}
	return r, nil
}

// JoinKey appends name to a prefix, inserting "/" when needed.
func JoinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	if strings.HasSuffix(prefix, "/") {
		return prefix + name
	}
	return prefix + "/" + name
}

// BaseName returns the last path segment of key ("" when key ends in "/").
func BaseName(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// NormalizePrefix ensures a non-empty prefix ends with "/" — the aws rule for
// `--recursive` targets, so `rm s3://b/path --recursive` does not touch
// `path2/`.
func NormalizePrefix(prefix string) string {
	if prefix == "" || strings.HasSuffix(prefix, "/") {
		return prefix
	}
	return prefix + "/"
}

// DescribeRef renders a ref for messages, always in s3:// form for remotes.
func DescribeRef(r Ref) string {
	if r.Stdio {
		return "-"
	}
	return r.String()
}

// ErrUsagef is a convenience for usage errors.
func ErrUsagef(format string, a ...interface{}) error {
	return exitcode.Errorf(exitcode.Usage, format, a...)
}

// bucketNotFound formats the "no such bucket" error shared by the resolver.
func bucketNotFound(ref, project string) error {
	if project != "" {
		return exitcode.Errorf(exitcode.NotFound, "bucket %q not found in project %s; run 'lsh s3 list' to see available buckets", ref, project)
	}
	return exitcode.Errorf(exitcode.NotFound, "bucket %q not found; run 'lsh s3 list' to see available buckets", ref)
}

var _ = fmt.Sprintf
