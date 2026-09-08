package s3

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Shared machinery for cp, mv and sync: operand classification, the aws
// destination rules, local/remote enumeration, the transfer loop and its
// output. Nothing in this file depends on cobra except the flag registration
// helpers at the bottom, so the core can be tested against s3test.

// Transfer operation names (also the aws output prefixes).
const (
	opUpload   = "upload"
	opDownload = "download"
	opCopy     = "copy"
	opMove     = "move"
	opDelete   = "delete"
)

// stdinPartSize is the smallest multipart part size used for stdin uploads
// (minio's default). S3 allows 10000 parts, so it covers streams up to
// stdinMaxPlainSize; --expected-size grows the parts beyond that.
const stdinPartSize = 16 * 1024 * 1024

// stdinMaxParts is the S3 limit on parts per multipart upload.
const stdinMaxParts = 10000

// stdinMaxPlainSize is the largest stream that fits in 10000 parts of
// stdinPartSize (160 GiB) without --expected-size.
const stdinMaxPlainSize = int64(stdinPartSize) * stdinMaxParts

// partialSuffix marks a download in progress; the file is renamed on success.
const partialSuffix = ".lsh-partial"

// transferOptions carries every knob the transfer core understands. The
// cobra layer fills it from flags; tests build it directly.
type transferOptions struct {
	Recursive bool
	Filters   *objectstorage.Filters

	ContentType        string
	CacheControl       string
	ContentEncoding    string
	ContentDisposition string
	ContentLanguage    string
	Expires            time.Time
	Metadata           map[string]string
	NoGuessMime        bool

	NoOverwrite    bool
	ExpectedSize   int64
	FollowSymlinks bool

	// Move deletes the source after a successful copy (mv).
	Move bool
	// DryRun plans and prints without writing anything.
	DryRun bool

	// Human prints aws-style lines to Out; otherwise results are only
	// collected for the structured renderer.
	Human          bool
	Quiet          bool
	OnlyShowErrors bool
	// ShowProgress enables the single-line progress meter on Hints.
	ShowProgress bool

	// Stdin/Stdout back the "-" operands (defaults: os.Stdin/os.Stdout).
	Stdin  io.Reader
	Stdout io.Writer
}

func (o *transferOptions) showLines() bool {
	return o.Human && !o.Quiet && !o.OnlyShowErrors
}

func (o *transferOptions) stdin() io.Reader {
	if o.Stdin != nil {
		return o.Stdin
	}
	return os.Stdin
}

func (o *transferOptions) stdout() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return os.Stdout
}

// endpoint is one side of a transfer: a parsed operand plus, for remote
// references, the open bucket session.
type endpoint struct {
	Ref  objectstorage.Ref
	Sess *Session
}

func (e endpoint) remote() bool { return e.Ref.Remote }
func (e endpoint) stdio() bool  { return e.Ref.Stdio }
func (e endpoint) local() bool  { return !e.Ref.Remote && !e.Ref.Stdio }

// remoteURI renders s3://<token>/<key> using the bucket token as typed.
func (e endpoint) remoteURI(key string) string { return e.Ref.WithKey(key).String() }

// transferItem is one planned operation.
type transferItem struct {
	Op string
	// Display strings for output lines.
	Src, Dst string
	// SrcPath/DstPath are local paths; SrcKey/DstKey are object keys. The
	// unused pair is empty. Stdio sides have both empty.
	SrcPath, SrcKey string
	DstPath, DstKey string
	// Size is known for local sources and listed/stat'ed remote sources.
	Size    int64
	ModTime time.Time
}

// transferPlan is the outcome of planning: the two endpoints and the items.
type transferPlan struct {
	Src, Dst endpoint
	Items    []transferItem
	// Deletes are destination entries removed by `sync --delete`.
	Deletes []deleteItem
}

// deleteItem is one `sync --delete` removal.
type deleteItem struct {
	Display string
	Path    string // local
	Key     string // remote
}

// entry is a file or object discovered while enumerating a side, keyed by
// its path relative to the source directory / prefix (forward slashes).
type entry struct {
	Rel     string
	Path    string // local absolute-ish path (as joined from the operand)
	Key     string // remote key
	Size    int64
	ModTime time.Time
}

// ---------------------------------------------------------------------------
// Operand validation

// classifyOperands validates the source/destination combination shared by
// cp, mv and sync and returns the parsed refs.
func classifyOperands(srcArg, dstArg string, move, sync bool) (objectstorage.Ref, objectstorage.Ref, error) {
	src, err := objectstorage.ParseTransferArg(srcArg)
	if err != nil {
		return src, objectstorage.Ref{}, err
	}
	dst, err := objectstorage.ParseTransferArg(dstArg)
	if err != nil {
		return src, dst, err
	}
	switch {
	case !src.Remote && !dst.Remote:
		if src.Stdio || dst.Stdio {
			return src, dst, objectstorage.ErrUsagef("one of <source> and <destination> must be an s3:// location")
		}
		return src, dst, objectstorage.ErrUsagef("both operands are local paths; at least one must be an s3:// location (use cp(1) for local copies)")
	case src.Stdio && sync, dst.Stdio && sync:
		return src, dst, objectstorage.ErrUsagef("sync does not support '-' (stdin/stdout)")
	case src.Stdio && move, dst.Stdio && move:
		return src, dst, objectstorage.ErrUsagef("mv does not support '-' (stdin/stdout); use cp")
	}
	return src, dst, nil
}

// sameEndpoint reports whether two buckets live on the same S3 endpoint.
func sameEndpoint(a, b *objectstorage.Bucket) bool {
	norm := func(s string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "/")) }
	return norm(a.Endpoint) == norm(b.Endpoint)
}

// checkCrossEndpoint rejects server-side copies between different endpoints.
func checkCrossEndpoint(src, dst endpoint) error {
	if !src.remote() || !dst.remote() {
		return nil
	}
	if sameEndpoint(src.Sess.Bucket, dst.Sess.Bucket) {
		return nil
	}
	return objectstorage.ErrUsagef("buckets %s (%s) and %s (%s) are on different endpoints; server-side copy is only possible within one endpoint — download and re-upload, or use rclone with 'lsh s3 configure export --format rclone'",
		src.Sess.Bucket.Display(), src.Sess.Bucket.Endpoint, dst.Sess.Bucket.Display(), dst.Sess.Bucket.Endpoint)
}

// needsStreamedCopy reports whether a failed server-side copy should be retried
// by streaming the object. Only an authorization failure between two buckets
// that resolved different credentials qualifies: any other error (missing
// object, quota, backend outage) would fail the same way twice.
func needsStreamedCopy(src, dst endpoint, err error) bool {
	if !src.remote() || !dst.remote() {
		return false
	}
	if minio.ToErrorResponse(err).Code != "AccessDenied" {
		return false
	}
	return src.Sess.Cred.AccessKeyID != dst.Sess.Cred.AccessKeyID
}

// streamCopy copies one object through the client: GET with the source session,
// PUT with the destination one. It is the fallback for buckets a single
// credential cannot span, so cp/mv/sync keep working instead of reporting an
// access denial the user cannot act on.
func streamCopy(ctx context.Context, src, dst endpoint, item transferItem, opts *transferOptions, res *objectstorage.TransferResult, hints io.Writer, label string) error {
	obj, err := src.Sess.Client.GetObject(ctx, src.Sess.Bucket.BucketName, item.SrcKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	stat, err := obj.Stat()
	if err != nil {
		// The read is the source's; blaming the destination (as the shared
		// humanizer would) points at the wrong bucket and the wrong key.
		return src.Sess.humanize(err)
	}
	meter := meterFor(opts, hints, label, stat.Size)
	defer meter.finish()
	put := streamedPutOptions(stat, opts, progressReader(meter))
	contentType := put.ContentType
	info, err := dst.Sess.Client.PutObject(ctx, dst.Sess.Bucket.BucketName, item.DstKey, obj, stat.Size, put)
	if err != nil {
		return dst.Sess.humanize(err)
	}
	res.ETag, res.ContentType = strings.Trim(info.ETag, `"`), contentType
	if info.Size > 0 {
		res.Size = info.Size
	}
	return nil
}

// streamedPutOptions makes a streamed copy keep what a server-side copy would
// have kept: the source's user metadata and its caching/content headers. The
// explicit flags still win — putOptions applies them first and this only fills
// what they left empty — so a cross-credential copy does not silently rewrite
// an object's metadata.
func streamedPutOptions(stat minio.ObjectInfo, opts *transferOptions, progress io.Reader) minio.PutObjectOptions {
	put := putOptions(firstNonEmptyStr(opts.ContentType, stat.ContentType), opts, progress)
	if len(put.UserMetadata) == 0 {
		put.UserMetadata = userMetadataOf(stat.Metadata)
	}
	if put.CacheControl == "" {
		put.CacheControl = stat.Metadata.Get("Cache-Control")
	}
	if put.ContentEncoding == "" {
		put.ContentEncoding = firstNonEmptyStr(stat.ContentEncoding, stat.Metadata.Get("Content-Encoding"))
	}
	if put.ContentDisposition == "" {
		put.ContentDisposition = stat.Metadata.Get("Content-Disposition")
	}
	if put.ContentLanguage == "" {
		put.ContentLanguage = stat.Metadata.Get("Content-Language")
	}
	if put.Expires.IsZero() {
		put.Expires = stat.Expires
	}
	return put
}

// userMetadataOf extracts the x-amz-meta-* response headers, stripped of the
// prefix. ObjectInfo.UserMetadata only carries them against MinIO servers, so
// the raw headers are the portable source.
func userMetadataOf(h http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range h {
		lower := strings.ToLower(key)
		if name := strings.TrimPrefix(lower, "x-amz-meta-"); name != lower && len(values) > 0 {
			out[name] = values[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// sameObject reports whether both endpoints name the same object.
func sameObject(src, dst endpoint, srcKey, dstKey string) bool {
	if !src.remote() || !dst.remote() {
		return false
	}
	return sameEndpoint(src.Sess.Bucket, dst.Sess.Bucket) &&
		src.Sess.Bucket.BucketName == dst.Sess.Bucket.BucketName && srcKey == dstKey
}

// ---------------------------------------------------------------------------
// Planning (cp / mv)

// buildPlan enumerates the source and applies the aws destination rules. It
// only performs read-only calls (stat, list), so it is safe under --dry-run.
func buildPlan(ctx context.Context, src, dst endpoint, opts *transferOptions) (*transferPlan, error) {
	plan := &transferPlan{Src: src, Dst: dst}
	op := opFor(src, dst, opts.Move)

	if err := checkCrossEndpoint(src, dst); err != nil {
		return nil, err
	}

	switch {
	case src.stdio():
		if opts.Recursive {
			return nil, objectstorage.ErrUsagef("--recursive cannot be combined with stdin ('-')")
		}
		if dst.Ref.IsDir() {
			return nil, objectstorage.ErrUsagef("%s names a prefix; uploading from stdin needs a full object key (s3://<bucket>/<key>)", dst.Ref)
		}
		plan.Items = append(plan.Items, transferItem{Op: op, Src: "-", Dst: dst.remoteURI(dst.Ref.Key), DstKey: dst.Ref.Key, Size: opts.ExpectedSize})
		return plan, nil

	case dst.stdio():
		if opts.Recursive {
			return nil, objectstorage.ErrUsagef("--recursive cannot be combined with stdout ('-')")
		}
		if src.Ref.Key == "" || strings.HasSuffix(src.Ref.Key, "/") {
			return nil, objectstorage.ErrUsagef("%s names a prefix, not an object; streaming to stdout needs a single object key", src.Ref)
		}
		info, err := src.Sess.Client.StatObject(ctx, src.Sess.Bucket.BucketName, src.Ref.Key, minio.StatObjectOptions{})
		if err != nil {
			return nil, src.Sess.humanize(err)
		}
		plan.Items = append(plan.Items, transferItem{Op: op, Src: src.remoteURI(src.Ref.Key), Dst: "-", SrcKey: src.Ref.Key, Size: info.Size, ModTime: info.LastModified})
		return plan, nil

	case opts.Recursive:
		return buildRecursivePlan(ctx, plan, op, opts)
	}

	// Single object / file.
	var item transferItem
	item.Op = op
	switch {
	case src.local():
		st, err := os.Stat(src.Ref.Raw)
		if err != nil {
			return nil, localErr(err)
		}
		if st.IsDir() {
			return nil, objectstorage.ErrUsagef("%s is a directory; use --recursive to copy its contents", src.Ref.Raw)
		}
		name := filepath.Base(src.Ref.Raw)
		key := dst.Ref.Key
		if dst.Ref.IsDir() {
			key = objectstorage.JoinKey(dst.Ref.Key, name)
		}
		item.Src, item.SrcPath = displayLocal(src.Ref.Raw), src.Ref.Raw
		item.Dst, item.DstKey = dst.remoteURI(key), key
		item.Size, item.ModTime = st.Size(), st.ModTime()
		if !opts.Filters.Include(name) {
			return plan, nil
		}
	default: // remote source
		if src.Ref.Key == "" || strings.HasSuffix(src.Ref.Key, "/") {
			return nil, objectstorage.ErrUsagef("%s names a prefix, not an object; use --recursive to copy everything under it", src.Ref)
		}
		info, err := src.Sess.Client.StatObject(ctx, src.Sess.Bucket.BucketName, src.Ref.Key, minio.StatObjectOptions{})
		if err != nil {
			return nil, src.Sess.humanize(err)
		}
		name := objectstorage.BaseName(src.Ref.Key)
		item.Src, item.SrcKey = src.remoteURI(src.Ref.Key), src.Ref.Key
		item.Size, item.ModTime = info.Size, info.LastModified
		if dst.remote() {
			key := dst.Ref.Key
			if dst.Ref.IsDir() {
				key = objectstorage.JoinKey(dst.Ref.Key, name)
			}
			if sameObject(src, dst, src.Ref.Key, key) {
				return nil, objectstorage.ErrUsagef("source and destination are the same object (%s)", item.Src)
			}
			item.Dst, item.DstKey = dst.remoteURI(key), key
		} else {
			path, display, err := localDestination(dst.Ref.Raw, name)
			if err != nil {
				return nil, err
			}
			item.Dst, item.DstPath = display, path
		}
		if !opts.Filters.Include(name) {
			return plan, nil
		}
	}
	plan.Items = append(plan.Items, item)
	return plan, nil
}

// buildRecursivePlan handles --recursive: both sides are directories /
// prefixes and every relative path is mirrored.
func buildRecursivePlan(ctx context.Context, plan *transferPlan, op string, opts *transferOptions) (*transferPlan, error) {
	src, dst := plan.Src, plan.Dst
	entries, err := enumerate(ctx, src, opts.FollowSymlinks)
	if err != nil {
		return nil, err
	}
	dstPrefix := ""
	if dst.remote() {
		dstPrefix = objectstorage.NormalizePrefix(dst.Ref.Key)
	}
	for _, e := range entries {
		if !opts.Filters.Include(e.Rel) {
			continue
		}
		item := transferItem{Op: op, Size: e.Size, ModTime: e.ModTime}
		if src.remote() {
			item.Src, item.SrcKey = src.remoteURI(e.Key), e.Key
		} else {
			item.Src, item.SrcPath = displayLocal(e.Path), e.Path
		}
		if dst.remote() {
			key := dstPrefix + e.Rel
			if sameObject(src, dst, e.Key, key) {
				continue
			}
			item.Dst, item.DstKey = dst.remoteURI(key), key
		} else {
			path, display, err := localRelDestination(dst.Ref.Raw, e.Rel)
			if err != nil {
				return nil, err
			}
			item.Dst, item.DstPath = display, path
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

// opFor picks the output verb for a direction.
func opFor(src, dst endpoint, move bool) string {
	if move {
		return opMove
	}
	switch {
	case src.remote() && dst.remote():
		return opCopy
	case src.remote():
		return opDownload
	default:
		return opUpload
	}
}

// ---------------------------------------------------------------------------
// Enumeration

// enumerate lists a directory (local) or a prefix (remote) recursively and
// returns entries keyed by relative path, sorted.
func enumerate(ctx context.Context, e endpoint, followSymlinks bool) ([]entry, error) {
	var out []entry
	var err error
	if e.remote() {
		out, err = listRemote(ctx, e.Sess, objectstorage.NormalizePrefix(e.Ref.Key))
	} else {
		out, err = walkLocal(e.Ref.Raw, followSymlinks)
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// listRemote lists every object under prefix. Directory markers (keys ending
// in "/") are skipped like aws does.
func listRemote(ctx context.Context, sess *Session, prefix string) ([]entry, error) {
	var out []entry
	for info := range sess.Client.ListObjects(ctx, sess.Bucket.BucketName, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if info.Err != nil {
			return nil, sess.humanize(info.Err)
		}
		if strings.HasSuffix(info.Key, "/") || len(info.Key) <= len(prefix) {
			continue
		}
		out = append(out, entry{Rel: info.Key[len(prefix):], Key: info.Key, Size: info.Size, ModTime: info.LastModified})
	}
	return out, nil
}

// walkLocal lists the regular files under root. Symlinks are followed (with
// loop protection) when follow is true and ignored otherwise.
func walkLocal(root string, follow bool) ([]entry, error) {
	st, err := os.Stat(root)
	if err != nil {
		return nil, localErr(err)
	}
	if !st.IsDir() {
		return nil, objectstorage.ErrUsagef("%s is not a directory", root)
	}
	visited := map[string]bool{}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		visited[real] = true
	}
	var out []entry
	var walk func(dir, relPrefix string) error
	walk = func(dir, relPrefix string) error {
		des, err := os.ReadDir(dir)
		if err != nil {
			return localErr(err)
		}
		for _, d := range des {
			path := filepath.Join(dir, d.Name())
			rel := relPrefix + d.Name()
			info, err := d.Info()
			if err != nil {
				return localErr(err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if !follow {
					continue
				}
				info, err = os.Stat(path)
				if err != nil {
					objectstorage.Warnf("skipping %s: %v", path, err)
					continue
				}
				if info.IsDir() {
					real, err := filepath.EvalSymlinks(path)
					if err != nil || visited[real] {
						continue
					}
					visited[real] = true
				}
			}
			switch {
			case info.IsDir():
				if err := walk(path, rel+"/"); err != nil {
					return err
				}
			case info.Mode().IsRegular():
				out = append(out, entry{Rel: rel, Path: path, Size: info.Size(), ModTime: info.ModTime()})
			}
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Local path helpers

// displayLocal renders a local path the way aws prints it (os.path.relpath):
// relative to the working directory when possible, with a leading "./" only
// when the result has no directory component ("./dump.sql", "sub/b.txt",
// "../other/x"). Every local side of an output line (sources, single and
// recursive destinations, sync deletes) goes through it.
func displayLocal(path string) string {
	if cwd, err := os.Getwd(); err == nil {
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, path)
		}
		// Rel fails across Windows volumes; the absolute path is shown then.
		if rel, err := filepath.Rel(cwd, abs); err == nil {
			path = rel
		}
	}
	if path == "." || path == ".." || strings.ContainsRune(path, filepath.Separator) || filepath.VolumeName(path) != "" {
		return path
	}
	return "." + string(filepath.Separator) + path
}

// endsWithSeparator reports whether a path ends with a path separator.
func endsWithSeparator(p string) bool {
	return strings.HasSuffix(p, "/") || strings.HasSuffix(p, string(filepath.Separator))
}

// escapeErr is the exit-7 error for a key that would be written outside the
// destination directory.
func escapeErr(key string) error {
	return exitcode.Errorf(exitcode.Refused, "refusing to write %q: the key escapes the destination directory", key)
}

// safeLocalName validates the file name appended to a destination directory
// for a single download: after FromSlash+Clean it must be a plain name (no
// separator, not "." or "..", no volume name), otherwise the key would land
// outside the directory.
func safeLocalName(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.ContainsRune(clean, filepath.Separator) || filepath.VolumeName(clean) != "" {
		return "", escapeErr(name)
	}
	return clean, nil
}

// localDestination applies the aws rule for a single download: an existing
// directory or a trailing separator means "put <name> inside", anything else
// is the literal file path. It returns the path to write and the display
// form; a name that would escape the directory is refused (exit 7).
func localDestination(dst, name string) (path, display string, err error) {
	inside := endsWithSeparator(dst)
	if !inside {
		if st, statErr := os.Stat(dst); statErr == nil && st.IsDir() {
			inside = true
		}
	}
	if !inside {
		return dst, displayLocal(dst), nil
	}
	clean, err := safeLocalName(name)
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(dst, clean)
	return path, displayLocal(path), nil
}

// localRelDestination joins a relative key path under the destination
// directory for recursive downloads, refusing paths that escape it.
func localRelDestination(dstDir, rel string) (path, display string, err error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
		return "", "", escapeErr(rel)
	}
	path = filepath.Join(dstDir, clean)
	return path, displayLocal(path), nil
}

// localErr wraps a filesystem error with an exit code.
func localErr(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return exitcode.New(exitcode.NotFound, err)
	case errors.Is(err, os.ErrPermission):
		return exitcode.New(exitcode.Permission, err)
	}
	return exitcode.New(exitcode.Generic, err)
}

// ---------------------------------------------------------------------------
// Content type and headers

// guessContentType implements the cp rules: explicit flag, else extension,
// else sniffing the first bytes, else application/octet-stream.
// --no-guess-mime-type yields binary/octet-stream (like aws).
func guessContentType(path string, opts *transferOptions) string {
	if opts.ContentType != "" {
		return opts.ContentType
	}
	if opts.NoGuessMime {
		return "binary/octet-stream"
	}
	if ext := filepath.Ext(path); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

// putOptions builds the minio options shared by uploads.
func putOptions(contentType string, opts *transferOptions, progress io.Reader) minio.PutObjectOptions {
	return minio.PutObjectOptions{
		ContentType:        contentType,
		UserMetadata:       opts.Metadata,
		CacheControl:       opts.CacheControl,
		ContentEncoding:    opts.ContentEncoding,
		ContentDisposition: opts.ContentDisposition,
		ContentLanguage:    opts.ContentLanguage,
		Expires:            opts.Expires,
		Progress:           progress,
	}
}

// parseMetadata parses the aws shorthand `k=v,k2=v2` (repeatable, merged).
func parseMetadata(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, v := range values {
		for _, pair := range strings.Split(v, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			k, val, ok := strings.Cut(pair, "=")
			k = strings.TrimSpace(k)
			if !ok || k == "" {
				return nil, objectstorage.ErrUsagef("invalid --metadata %q: expected key=value[,key2=value2]", pair)
			}
			out[k] = strings.TrimSpace(val)
		}
	}
	return out, nil
}

// parseExpires accepts RFC3339 or YYYY-MM-DD.
func parseExpires(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, objectstorage.ErrUsagef("invalid --expires %q: use RFC3339 (2026-09-07T15:04:05Z) or YYYY-MM-DD", s)
}

// ---------------------------------------------------------------------------
// Progress

// progressMeter is a minio Progress reader that rewrites one stderr line
// every ~200ms. It also works as a counting writer wrapper for downloads.
type progressMeter struct {
	w     io.Writer
	label string
	total int64
	done  atomic.Int64

	mu       sync.Mutex // guards last and rendered (minio uploads parts concurrently)
	last     time.Time
	rendered bool
}

func newProgressMeter(w io.Writer, label string, total int64) *progressMeter {
	return &progressMeter{w: w, label: label, total: total}
}

// Read implements minio's progress hook contract: every call reports len(b)
// bytes transferred.
func (p *progressMeter) Read(b []byte) (int, error) {
	p.add(int64(len(b)))
	return len(b), nil
}

func (p *progressMeter) add(n int64) {
	if p == nil {
		return
	}
	done := p.done.Add(n)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	// total is 0 for a stdin upload: there is no final frame to force, so the
	// throttle applies to every update.
	if now.Sub(p.last) < 200*time.Millisecond && (p.total <= 0 || done < p.total) {
		return
	}
	p.last = now
	p.render(done)
}

// render draws the line; the caller holds p.mu.
func (p *progressMeter) render(done int64) {
	p.rendered = true
	if p.total > 0 {
		pct := float64(done) / float64(p.total) * 100
		if pct > 100 {
			pct = 100
		}
		fmt.Fprintf(p.w, "\r\033[K%s: %s / %s (%.0f%%)", p.label, objectstorage.HumanSize(done), objectstorage.HumanSize(p.total), pct)
		return
	}
	fmt.Fprintf(p.w, "\r\033[K%s: %s", p.label, objectstorage.HumanSize(done))
}

// finish clears the progress line.
func (p *progressMeter) finish() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rendered {
		fmt.Fprint(p.w, "\r\033[K")
	}
}

// countingWriter feeds a progress meter while copying downloads.
type countingWriter struct {
	w io.Writer
	p *progressMeter
}

func (c *countingWriter) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.p.add(int64(n))
	return n, err
}

// meterFor returns a progress meter when enabled, nil otherwise. A nil
// *progressMeter must never be passed as minio Progress (it is an interface).
func meterFor(opts *transferOptions, hints io.Writer, label string, total int64) *progressMeter {
	if !opts.ShowProgress {
		return nil
	}
	return newProgressMeter(hints, label, total)
}

func progressReader(p *progressMeter) io.Reader {
	if p == nil {
		return nil
	}
	return p
}

// ---------------------------------------------------------------------------
// Execution

// runTransfers executes a plan. Human lines go to out, hints/progress/errors
// to hints. It returns the collected results and, when something failed, an
// exit-coded error (the item's own error for a single-item plan, exit 1
// otherwise). Deletes from sync plans are executed after the copies, and only
// when every copy succeeded (see below).
func runTransfers(ctx context.Context, plan *transferPlan, opts *transferOptions, out, hints io.Writer) ([]objectstorage.TransferResult, error) {
	// total is the planned size and never changes: it decides where a failure
	// is printed (emit) and whether the caller prints the item's own error.
	// attempted excludes deletions that were skipped, so the failure summary
	// counts only what ran.
	total := len(plan.Items) + len(plan.Deletes)
	attempted := total
	results := make([]objectstorage.TransferResult, 0, total)
	failures := 0
	var firstErr error

	emit := func(res objectstorage.TransferResult) {
		results = append(results, res)
		if res.Error != "" {
			// Failures always go to stderr, even with --quiet. A single-item
			// plan returns its error instead, so it is printed once.
			if total > 1 {
				fmt.Fprintln(hints, humanLine(res))
			}
			return
		}
		if !opts.showLines() {
			return
		}
		if plan.Dst.stdio() {
			// stdout carries the object itself, so lines are suppressed —
			// except under --dry-run, where nothing is streamed and the plan
			// would otherwise be invisible.
			if opts.DryRun {
				fmt.Fprintln(hints, humanLine(res))
			}
			return
		}
		fmt.Fprintln(out, humanLine(res))
	}

	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			return results, exitcode.Errorf(exitcode.Interrupted, "interrupted")
		}
		res := objectstorage.TransferResult{Op: item.Op, Source: item.Src, Destination: item.Dst, Size: item.Size, DryRun: opts.DryRun}
		skip, err := shouldSkipExisting(ctx, plan, item, opts, hints)
		if err == nil && skip {
			continue
		}
		if err == nil && !opts.DryRun {
			err = transferOne(ctx, plan, item, opts, &res, hints)
		}
		if err != nil {
			if ctx.Err() != nil {
				return results, exitcode.Errorf(exitcode.Interrupted, "interrupted")
			}
			err = humanizeFor(plan, err)
			res.Error = err.Error()
			failures++
			if firstErr == nil {
				firstErr = err
			}
		}
		emit(res)
	}

	// `sync --delete` only plans deletions for keys the source does not have,
	// so applying them after a failed copy would leave the destination with
	// neither the object that failed to transfer nor the one that was about to
	// be pruned. Skip them instead: re-running the sync applies them once the
	// transfers succeed, and nothing has been lost in the meantime.
	deletes := plan.Deletes
	if failures > 0 && len(deletes) > 0 {
		fmt.Fprintf(hints, "warning: %d planned deletion(s) not applied because %d transfer(s) failed; the destination is not in sync — re-run once the transfers succeed\n", len(deletes), failures)
		attempted -= len(deletes)
		deletes = nil
	}

	for _, del := range deletes {
		if err := ctx.Err(); err != nil {
			return results, exitcode.Errorf(exitcode.Interrupted, "interrupted")
		}
		res := objectstorage.TransferResult{Op: opDelete, Destination: del.Display, DryRun: opts.DryRun}
		if !opts.DryRun {
			if err := deleteOne(ctx, plan.Dst, del); err != nil {
				if ctx.Err() != nil {
					return results, exitcode.Errorf(exitcode.Interrupted, "interrupted")
				}
				err = humanizeFor(plan, err)
				res.Error = err.Error()
				failures++
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		emit(res)
	}

	if failures == 0 {
		return results, nil
	}
	if total == 1 {
		return results, firstErr
	}
	return results, exitcode.Errorf(exitcode.Generic, "%d of %d transfers failed", failures, attempted)
}

// humanLine renders a result as an aws-style line; deletes have no source.
func humanLine(res objectstorage.TransferResult) string {
	if res.Op != opDelete {
		return res.HumanLine()
	}
	prefix := ""
	if res.DryRun {
		prefix = "(dryrun) "
	}
	if res.Error != "" {
		return fmt.Sprintf("%sdelete failed: %s: %s", prefix, res.Destination, res.Error)
	}
	return fmt.Sprintf("%sdelete: %s", prefix, res.Destination)
}

// humanizeFor maps an error through the most relevant session.
func humanizeFor(plan *transferPlan, err error) error {
	var already *exitcode.Error
	if errors.As(err, &already) {
		return err
	}
	if plan.Dst.remote() {
		return plan.Dst.Sess.humanize(err)
	}
	if plan.Src.remote() {
		return plan.Src.Sess.humanize(err)
	}
	return objectstorage.Humanize(err, nil, nil)
}

// shouldSkipExisting implements --no-overwrite: an existing destination is
// left alone and reported on hints.
func shouldSkipExisting(ctx context.Context, plan *transferPlan, item transferItem, opts *transferOptions, hints io.Writer) (bool, error) {
	if !opts.NoOverwrite || plan.Dst.stdio() {
		return false, nil
	}
	exists := false
	if plan.Dst.remote() {
		_, err := plan.Dst.Sess.Client.StatObject(ctx, plan.Dst.Sess.Bucket.BucketName, item.DstKey, minio.StatObjectOptions{})
		switch {
		case err == nil:
			exists = true
		case minio.ToErrorResponse(err).StatusCode == http.StatusNotFound:
		default:
			return false, err
		}
	} else {
		_, err := os.Stat(item.DstPath)
		switch {
		case err == nil:
			exists = true
		case errors.Is(err, os.ErrNotExist):
		default:
			return false, err
		}
	}
	if exists {
		fmt.Fprintf(hints, "skip: %s already exists (--no-overwrite)\n", item.Dst)
	}
	return exists, nil
}

// transferOne performs a single planned item and fills the result.
func transferOne(ctx context.Context, plan *transferPlan, item transferItem, opts *transferOptions, res *objectstorage.TransferResult, hints io.Writer) error {
	src, dst := plan.Src, plan.Dst
	label := fmt.Sprintf("%s -> %s", item.Src, item.Dst)
	switch {
	case src.stdio():
		return uploadStdin(ctx, dst, item, opts, res, hints)
	case dst.stdio():
		return download(ctx, src, item, opts, res, opts.stdout(), nil)
	case src.local() && dst.remote():
		meter := meterFor(opts, hints, label, item.Size)
		defer meter.finish()
		ct := guessContentType(item.SrcPath, opts)
		info, err := dst.Sess.Client.FPutObject(ctx, dst.Sess.Bucket.BucketName, item.DstKey, item.SrcPath, putOptions(ct, opts, progressReader(meter)))
		if err != nil {
			return err
		}
		res.ETag, res.ContentType = strings.Trim(info.ETag, `"`), ct
		if info.Size > 0 {
			res.Size = info.Size
		}
		if opts.Move {
			return os.Remove(item.SrcPath)
		}
		return nil
	case src.remote() && dst.local():
		meter := meterFor(opts, hints, label, item.Size)
		defer meter.finish()
		if err := download(ctx, src, item, opts, res, nil, meter); err != nil {
			return err
		}
		if opts.Move {
			return src.Sess.Client.RemoveObject(ctx, src.Sess.Bucket.BucketName, item.SrcKey, minio.RemoveObjectOptions{})
		}
		return nil
	default: // remote -> remote
		dstOpts := minio.CopyDestOptions{Bucket: dst.Sess.Bucket.BucketName, Object: item.DstKey}
		if len(opts.Metadata) > 0 || opts.ContentType != "" || opts.CacheControl != "" || opts.ContentEncoding != "" || opts.ContentDisposition != "" || opts.ContentLanguage != "" || !opts.Expires.IsZero() {
			dstOpts.ReplaceMetadata = true
			dstOpts.UserMetadata = opts.Metadata
			dstOpts.ContentType = opts.ContentType
			dstOpts.CacheControl = opts.CacheControl
			dstOpts.ContentEncoding = opts.ContentEncoding
			dstOpts.ContentDisposition = opts.ContentDisposition
			dstOpts.ContentLanguage = opts.ContentLanguage
			dstOpts.Expires = opts.Expires
		}
		info, err := dst.Sess.Client.CopyObject(ctx, dstOpts, minio.CopySrcOptions{Bucket: src.Sess.Bucket.BucketName, Object: item.SrcKey})
		switch {
		case err == nil:
			res.ETag = strings.Trim(info.ETag, `"`)
		case needsStreamedCopy(src, dst, err):
			// A CopyObject carries a single identity: it is signed with the
			// destination credential and the backend reads the source as that
			// same identity. When the two buckets resolved different keys the
			// destination key cannot read the source, so the object is streamed
			// through this machine instead — read with the source credential,
			// written with the destination one.
			fmt.Fprintf(hints, "%s and %s resolved different access keys, so the copy streams through this machine instead of the backend\n", src.Sess.Bucket.Display(), dst.Sess.Bucket.Display())
			if streamErr := streamCopy(ctx, src, dst, item, opts, res, hints, label); streamErr != nil {
				return streamErr
			}
		default:
			return err
		}
		if opts.Move {
			return src.Sess.Client.RemoveObject(ctx, src.Sess.Bucket.BucketName, item.SrcKey, minio.RemoveObjectOptions{})
		}
		return nil
	}
}

// stdinPartSizeFor picks the multipart part size for a stdin upload. Without
// --expected-size it is stdinPartSize; with it, the size minio would use for
// an object of that length (so streams above 160 GiB still fit in 10000
// parts), never below stdinPartSize. The value only shapes the parts: the
// upload itself is always length-unknown and succeeds whatever the real size.
func stdinPartSizeFor(expected int64) uint64 {
	if expected <= 0 {
		return stdinPartSize
	}
	_, partSize, _, err := minio.OptimalPartInfo(expected, 0)
	if err != nil || partSize < stdinPartSize {
		return stdinPartSize
	}
	return uint64(partSize)
}

// uploadStdin streams stdin as a multipart upload of unknown length. minio
// reads one part at a time into a buffer of the part size, so memory use is
// one part; --expected-size only sizes the parts and the progress total.
func uploadStdin(ctx context.Context, dst endpoint, item transferItem, opts *transferOptions, res *objectstorage.TransferResult, hints io.Writer) error {
	rd := bufio.NewReaderSize(opts.stdin(), 512)
	ct := opts.ContentType
	switch {
	case ct != "":
	case opts.NoGuessMime:
		ct = "binary/octet-stream"
	default:
		ct = "application/octet-stream"
		if ext := filepath.Ext(item.DstKey); ext != "" {
			if byExt := mime.TypeByExtension(ext); byExt != "" {
				ct = byExt
			}
		}
		if ct == "application/octet-stream" {
			if head, _ := rd.Peek(512); len(head) > 0 {
				ct = http.DetectContentType(head)
			}
		}
	}
	partSize := stdinPartSizeFor(opts.ExpectedSize)
	if opts.ExpectedSize <= 0 {
		fmt.Fprintf(hints, "note: streaming from stdin in %s parts (streams up to %s; pass --expected-size <bytes> for larger ones)\n", objectstorage.HumanSize(stdinPartSize), objectstorage.HumanSize(stdinMaxPlainSize))
	}
	meter := meterFor(opts, hints, "- -> "+item.Dst, opts.ExpectedSize)
	defer meter.finish()
	po := putOptions(ct, opts, progressReader(meter))
	po.PartSize = partSize
	// Size -1: the real length is whatever stdin delivers. Passing the
	// expected size here would make minio fail on a shorter stream and
	// truncate a longer one.
	info, err := dst.Sess.Client.PutObject(ctx, dst.Sess.Bucket.BucketName, item.DstKey, rd, -1, po)
	if err != nil {
		return err
	}
	res.ETag, res.ContentType, res.Size = strings.Trim(info.ETag, `"`), ct, info.Size
	return nil
}

// download fetches one object either into w (stdout) or into item.DstPath
// through a temporary ".<name>.lsh-partial" file renamed on success. The
// local file's mtime is set to the object's LastModified so sync can compare.
func download(ctx context.Context, src endpoint, item transferItem, opts *transferOptions, res *objectstorage.TransferResult, w io.Writer, meter *progressMeter) error {
	obj, err := src.Sess.Client.GetObject(ctx, src.Sess.Bucket.BucketName, item.SrcKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	info, err := obj.Stat()
	if err != nil {
		return err
	}
	res.Size, res.ETag, res.ContentType = info.Size, strings.Trim(info.ETag, `"`), info.ContentType

	var body io.Reader = obj
	if w != nil {
		if meter != nil {
			_, err = io.Copy(&countingWriter{w: w, p: meter}, body)
		} else {
			_, err = io.Copy(w, body)
		}
		return err
	}

	dir := filepath.Dir(item.DstPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return localErr(err)
	}
	tmp := filepath.Join(dir, "."+filepath.Base(item.DstPath)+partialSuffix)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return localErr(err)
	}
	var dst io.Writer = f
	if meter != nil {
		dst = &countingWriter{w: f, p: meter}
	}
	if _, err := io.Copy(dst, body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return localErr(err)
	}
	if err := os.Rename(tmp, item.DstPath); err != nil {
		os.Remove(tmp)
		return localErr(err)
	}
	if !info.LastModified.IsZero() {
		_ = os.Chtimes(item.DstPath, info.LastModified, info.LastModified)
	}
	return nil
}

// deleteOne removes a destination entry (sync --delete).
func deleteOne(ctx context.Context, dst endpoint, del deleteItem) error {
	if dst.remote() {
		return dst.Sess.Client.RemoveObject(ctx, dst.Sess.Bucket.BucketName, del.Key, minio.RemoveObjectOptions{})
	}
	if err := os.Remove(del.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return localErr(err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Cobra glue shared by cp, mv and sync

// transferFlags holds the raw flag values before conversion.
type transferFlags struct {
	recursive          bool
	filters            objectstorage.Filters
	contentType        string
	cacheControl       string
	contentEncoding    string
	contentDisposition string
	contentLanguage    string
	expires            string
	metadata           []string
	noGuessMime        bool
	noOverwrite        bool
	expectedSize       int64
	quiet              bool
	onlyShowErrors     bool
	noProgress         bool
	followSymlinks     bool
	noFollowSymlinks   bool
}

// addTransferFlags registers the flags shared by cp, mv and sync. recursive
// controls whether --recursive/-r is offered (sync is always recursive).
func addTransferFlags(cmd *cobra.Command, f *transferFlags, recursive bool) {
	fs := cmd.Flags()
	if recursive {
		fs.BoolVarP(&f.recursive, "recursive", "r", false, "copy every file/object under the source directory or prefix, mirroring relative paths")
	}
	objectstorage.FilterFlags(fs, &f.filters)
	fs.StringVar(&f.contentType, "content-type", "", "Content-Type to store (default: guessed from the extension, then the content)")
	fs.StringVar(&f.cacheControl, "cache-control", "", "Cache-Control header to store with the object")
	fs.StringVar(&f.contentEncoding, "content-encoding", "", "Content-Encoding header to store with the object")
	fs.StringVar(&f.contentDisposition, "content-disposition", "", "Content-Disposition header to store with the object")
	fs.StringVar(&f.contentLanguage, "content-language", "", "Content-Language header to store with the object")
	fs.StringVar(&f.expires, "expires", "", "Expires header (RFC3339 or YYYY-MM-DD)")
	fs.StringArrayVar(&f.metadata, "metadata", nil, "user metadata as key=value[,key2=value2] (repeatable, merged)")
	fs.BoolVar(&f.noGuessMime, "no-guess-mime-type", false, "store binary/octet-stream instead of guessing the Content-Type")
	fs.BoolVar(&f.noOverwrite, "no-overwrite", false, "skip destinations that already exist")
	fs.Int64Var(&f.expectedSize, "expected-size", 0, "approximate size in bytes of a stdin upload; only sizes the multipart parts (needed above 160 GiB) and the progress total, the upload succeeds whatever the real length")
	fs.BoolVarP(&f.quiet, "quiet", "q", false, "do not print the per-object lines")
	fs.BoolVar(&f.onlyShowErrors, "only-show-errors", false, "print only errors and warnings")
	fs.BoolVar(&f.noProgress, "no-progress", false, "do not show the progress meter")
	fs.BoolVar(&f.followSymlinks, "follow-symlinks", true, "follow symbolic links when enumerating local directories")
	fs.BoolVar(&f.noFollowSymlinks, "no-follow-symlinks", false, "ignore symbolic links when enumerating local directories")
	addProjectFlag(cmd, true, "project that owns the bucket(s) (disambiguates names shared across projects)")

	// `aws s3 cp ... --region x` gets the directed explanation (exit 2).
	rejectRegionFlag(cmd)
	unsupportedAWSFlags(cmd, map[string]string{
		"acl":                   "buckets are private; grant access with scoped access keys (lsh s3 access-keys create)",
		"grants":                "buckets are private; grant access with scoped access keys (lsh s3 access-keys create)",
		"sse":                   "server-side encryption is managed by the platform",
		"sse-c":                 "customer-provided encryption keys are not supported",
		"sse-c-key":             "customer-provided encryption keys are not supported",
		"sse-kms-key-id":        "KMS is not available on Latitude object storage",
		"sse-c-copy-source":     "customer-provided encryption keys are not supported",
		"sse-c-copy-source-key": "customer-provided encryption keys are not supported",
		"metadata-directive":    "server-side copies keep the source metadata; pass --metadata/--content-type to replace it",
		"copy-props":            "server-side copies always keep the source properties",
		"checksum-algorithm":    "the CLI uses Content-MD5; additional checksums are rejected by S3-compatible backends",
		"checksum-mode":         "checksum validation on download is not available",
		"request-payer":         "there is no requester-pays billing on Latitude",
		"storage-class":         "storage class is a bucket attribute on Latitude (standard|high_performance); set it with lsh s3 mb --storage-class",
	})
	if sse := fs.Lookup("sse"); sse != nil {
		sse.NoOptDefVal = "AES256"
	}
}

// buildTransferOptions converts flags into transferOptions.
func buildTransferOptions(cmd *cobra.Command, f *transferFlags) (*transferOptions, error) {
	meta, err := parseMetadata(f.metadata)
	if err != nil {
		return nil, err
	}
	expires, err := parseExpires(f.expires)
	if err != nil {
		return nil, err
	}
	if f.expectedSize < 0 {
		return nil, objectstorage.ErrUsagef("--expected-size must be a positive number of bytes")
	}
	follow := f.followSymlinks
	if f.noFollowSymlinks {
		follow = false
	}
	human := isHuman()
	return &transferOptions{
		Recursive:          f.recursive,
		Filters:            &f.filters,
		ContentType:        f.contentType,
		CacheControl:       f.cacheControl,
		ContentEncoding:    f.contentEncoding,
		ContentDisposition: f.contentDisposition,
		ContentLanguage:    f.contentLanguage,
		Expires:            expires,
		Metadata:           meta,
		NoGuessMime:        f.noGuessMime,
		NoOverwrite:        f.noOverwrite,
		ExpectedSize:       f.expectedSize,
		FollowSymlinks:     follow,
		DryRun:             dryRun(),
		Human:              human,
		Quiet:              f.quiet,
		OnlyShowErrors:     f.onlyShowErrors,
		ShowProgress:       human && !f.quiet && !f.onlyShowErrors && !f.noProgress && stderrIsTerminal(),
	}, nil
}

// stderrIsTerminal reports whether progress can be drawn.
func stderrIsTerminal() bool { return term.IsTerminal(int(os.Stderr.Fd())) }

// openEndpoints resolves the remote side(s) of a transfer. The destination
// always needs write permission; the source needs it only for mv.
func openEndpoints(ctx context.Context, cmd *cobra.Command, src, dst objectstorage.Ref, move bool) (endpoint, endpoint, error) {
	s, d := endpoint{Ref: src}, endpoint{Ref: dst}
	var err error
	if src.Remote {
		if s.Sess, err = openBucket(ctx, cmd, src.Bucket, move); err != nil {
			return s, d, err
		}
	}
	if !dst.Remote {
		return s, d, nil
	}
	switch {
	case s.Sess != nil && src.Bucket == dst.Bucket && move:
		// Same bucket and the source session already has write permission.
		d.Sess = s.Sess
	case s.Sess != nil && src.Bucket == dst.Bucket:
		// Same bucket: skip the second API lookup but select a write credential.
		d.Sess, err = openResolved(cmd, s.Sess.Bucket, true)
	default:
		d.Sess, err = openBucket(ctx, cmd, dst.Bucket, true)
	}
	return s, d, err
}

// finishTransfer renders structured output and returns the command error.
func finishTransfer(plan *transferPlan, opts *transferOptions, results []objectstorage.TransferResult, err error) error {
	if !opts.Human && !plan.Dst.stdio() {
		render(objectstorage.AsResponseData(results))
	}
	if err != nil {
		return printErr(err)
	}
	return nil
}

// runCopyCommand is the RunE body shared by cp and mv.
func runCopyCommand(cmd *cobra.Command, args []string, f *transferFlags, move bool) error {
	srcRef, dstRef, err := classifyOperands(args[0], args[1], move, false)
	if err != nil {
		return printErr(err)
	}
	opts, err := buildTransferOptions(cmd, f)
	if err != nil {
		return printErr(err)
	}
	opts.Move = move

	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	src, dst, err := openEndpoints(ctx, cmd, srcRef, dstRef, move)
	if err != nil {
		return printErr(err)
	}
	plan, err := buildPlan(ctx, src, dst, opts)
	if err != nil {
		return printErr(err)
	}
	if len(plan.Items) == 0 && opts.showLines() && !dst.stdio() {
		objectstorage.Hintf("nothing to %s", verbFor(move))
	}
	results, err := runTransfers(ctx, plan, opts, os.Stdout, os.Stderr)
	return finishTransfer(plan, opts, results, err)
}

func verbFor(move bool) string {
	if move {
		return "move"
	}
	return "copy"
}

// destinationRulesHelp documents the aws fileformat rules in --help.
const destinationRulesHelp = `Destination rules:
  s3:// destination ending in "/" or naming only the bucket  -> the source file name is appended
  s3:// destination without a trailing "/"                    -> the key is used literally
        (cp dump.sql s3://b/2026/09 creates the key "2026/09"; the output shows it)
  local destination that is an existing directory or ends in the path separator
                                                              -> the file name is appended (directories are created)
  any other local destination                                 -> literal file path
  --recursive forces directory semantics on both sides and mirrors paths relative to the source.

"-" reads the source from stdin or writes the destination to stdout (nothing else is printed on stdout).
Both operands remote -> server-side copy, only within the same endpoint.`
