package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

// rmBatchSize is the S3 multi-delete limit.
const rmBatchSize = 1000

// rmOptions are the parsed flags of `lsh s3 delete`.
type rmOptions struct {
	Recursive        bool
	All              bool
	MaxDelete        int
	Versions         bool
	VersionID        string
	BypassGovernance bool
	Quiet            bool
	OnlyShowErrors   bool
	DryRun           bool
	Filters          *objectstorage.Filters
}

// NewRmCmd builds `lsh s3 delete s3://bucket/key`.
func NewRmCmd() *cobra.Command {
	filters := &objectstorage.Filters{}
	cmd := newCmd(&cobra.Command{
		Use:     "delete s3://bucket/key",
		Aliases: []string{"rm", "delete-object", "remove"},
		GroupID: groupObjects,
		Short:   "Delete objects — the bucket stays (alias: rm)",
		Long: `Delete one object, or every object under a prefix with --recursive.

A single object is deleted without confirmation and the command exits 0 even
when the key does not exist (S3 deletes are idempotent). With --recursive the
prefix is normalized to end with "/", the matching objects are enumerated and
counted, and you are asked to confirm (or pass --yes). Deleting every object
in a bucket (s3://bucket with no prefix) additionally requires --all.

Filters (--exclude/--include) are shell globs evaluated against the key
relative to the prefix, in the order given; the last matching rule wins.

Object lock: buckets in COMPLIANCE mode are refused; GOVERNANCE mode requires
--bypass-governance-retention. Use --dry-run to print the plan without
deleting anything.`,
		Example: `  lsh s3 delete s3://backups/2026/09/dump.sql
  lsh s3 delete s3://logs/tmp/ --recursive --dry-run
  lsh s3 delete s3://logs/tmp/ --recursive --exclude "*" --include "*.log" --yes
  lsh s3 delete s3://logs --recursive --all --max-delete 5000 --yes
  lsh s3 delete s3://backups/dump.sql --version-id 3HL4kqtJlcpXroDTDmJ`,
		Args: cobra.ExactArgs(1),
	})
	f := cmd.Flags()
	f.BoolP("recursive", "r", false, "delete every object under the prefix (asks for confirmation)")
	f.Bool("all", false, "allow deleting every object in the bucket (required when no prefix is given)")
	f.Int("max-delete", 0, "refuse to delete more than N objects (0 = unlimited)")
	f.Bool("versions", false, "with --recursive: delete all versions and delete markers, not only the current objects")
	f.String("version-id", "", "delete this specific version of the object")
	f.Bool("bypass-governance-retention", false, "bypass GOVERNANCE object lock retention (requires a key allowed to do so)")
	f.BoolP("quiet", "q", false, "do not print the deleted objects")
	f.Bool("only-show-errors", false, "print only failures")
	objectstorage.FilterFlags(f, filters)
	addYesFlag(cmd)
	addProjectFlag(cmd, true, "project the bucket belongs to (disambiguates buckets with the same name)")
	addBucketFilterFlags(cmd)
	unsupportedAWSBoolFlags(cmd, map[string]string{
		"request-payer": "Latitude buckets have no requester-pays mode",
	})
	rejectRegionFlag(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		opts := rmOptions{Filters: filters, DryRun: dryRun()}
		opts.Recursive, _ = f.GetBool("recursive")
		opts.All, _ = f.GetBool("all")
		opts.MaxDelete, _ = f.GetInt("max-delete")
		opts.Versions, _ = f.GetBool("versions")
		opts.VersionID, _ = f.GetString("version-id")
		opts.BypassGovernance, _ = f.GetBool("bypass-governance-retention")
		opts.Quiet, _ = f.GetBool("quiet")
		opts.OnlyShowErrors, _ = f.GetBool("only-show-errors")
		yes, _ := f.GetBool(flagYes)

		ref, err := rmParseArgs(args[0], opts)
		if err != nil {
			return printErr(err)
		}

		ctx, stop := objectstorage.SignalContext(context.Background())
		defer stop()

		sess, err := openBucket(ctx, cmd, ref.Bucket, true)
		if err != nil {
			return printErr(err)
		}
		confirm := func(question string) error {
			return objectstorage.ConfirmOrRefuse(cmd, yes, question)
		}
		rows, err := runRm(ctx, sess, ref, opts, isHuman(), os.Stdout, confirm)
		if !isHuman() && len(rows) > 0 {
			render(objectstorage.AsResponseData(rows))
		}
		if err != nil {
			return printErr(err)
		}
		return nil
	}
	return cmd
}

// rmParseArgs parses the target and validates flag combinations.
func rmParseArgs(arg string, opts rmOptions) (objectstorage.Ref, error) {
	if opts.MaxDelete < 0 {
		return objectstorage.Ref{}, objectstorage.ErrUsagef("--max-delete must be zero or positive")
	}
	if !opts.Recursive {
		if opts.All {
			return objectstorage.Ref{}, objectstorage.ErrUsagef("--all only makes sense with --recursive")
		}
		if opts.Versions {
			return objectstorage.Ref{}, objectstorage.ErrUsagef("--versions requires --recursive; use --version-id to delete one version of an object")
		}
		// ObjectRef rejects bucket-only targets and keys ending in "/" with
		// the hint to add --recursive. A bare bucket gets a longer message:
		// the legacy `storage-objects rm <bkt_id>` deleted the bucket, and
		// that alias now lands here, so both destinations are spelled out.
		ref, err := objectstorage.ObjectRef(arg, false)
		if err != nil {
			if bucket, bucketErr := objectstorage.ParseBucketOnly(arg); bucketErr == nil {
				return ref, objectstorage.ErrUsagef("%s names a bucket, not an object; to delete its objects use 'lsh s3 delete s3://%s --recursive --all', to delete the bucket itself use 'lsh s3 delete-bucket s3://%s'", arg, bucket.Bucket, bucket.Bucket)
			}
		}
		return ref, err
	}
	if opts.VersionID != "" {
		return objectstorage.Ref{}, objectstorage.ErrUsagef("--version-id cannot be combined with --recursive; use --versions to delete every version under the prefix")
	}
	ref, err := objectstorage.ParseRemote(arg)
	if err != nil {
		return ref, err
	}
	ref.Key = objectstorage.NormalizePrefix(strings.TrimLeft(ref.Key, "/"))
	if ref.Key == "" && !opts.All {
		return ref, objectstorage.ErrUsagef("refusing to delete every object in s3://%s without --all", ref.Bucket)
	}
	return ref, nil
}

// runRm executes rm against an open session. Human lines go to out; the
// structured rows are returned for the caller to render. Errors carry exit
// codes (2 usage, 6 partial, 7 refused).
func runRm(ctx context.Context, sess *Session, ref objectstorage.Ref, opts rmOptions, human bool, out io.Writer, confirm func(question string) error) ([]objectstorage.DeleteResult, error) {
	if !opts.Recursive {
		return rmSingle(ctx, sess, ref.Key, opts, human, out)
	}
	return rmRecursive(ctx, sess, ref.Key, opts, human, out, confirm)
}

// rmBucketLabel is the bucket part of `s3://<bucket>/<key>` in the output:
// the display name, or the backend name when addressed by --endpoint-url.
func rmBucketLabel(b *objectstorage.Bucket) string {
	if b.Name != "" && !b.EndpointOverride {
		return b.Name
	}
	return b.BucketName
}

// rmCheckObjectLock enforces the object lock rules before any write.
func rmCheckObjectLock(b *objectstorage.Bucket, bypass bool) error {
	if !b.Locking {
		return nil
	}
	switch strings.ToUpper(b.RetentionMode) {
	case "COMPLIANCE":
		return exitcode.Errorf(exitcode.Refused, "bucket %s has object lock in COMPLIANCE mode; objects cannot be deleted until their retention expires", b.Display())
	case "GOVERNANCE":
		if !bypass {
			return exitcode.Errorf(exitcode.Refused, "bucket %s has object lock in GOVERNANCE mode; re-run with --bypass-governance-retention to delete retained objects", b.Display())
		}
	}
	return nil
}

// rmSingle deletes one object (or one version) without HEAD and without a
// prompt. A missing key is not an error.
func rmSingle(ctx context.Context, sess *Session, key string, opts rmOptions, human bool, out io.Writer) ([]objectstorage.DeleteResult, error) {
	if opts.VersionID != "" {
		// Deleting a specific version is permanent, so the lock rules apply;
		// without --version-id a versioned bucket only gets a delete marker.
		if err := rmCheckObjectLock(sess.Bucket, opts.BypassGovernance); err != nil {
			return nil, err
		}
	}
	res := objectstorage.DeleteResult{Bucket: rmBucketLabel(sess.Bucket), Key: key, VersionID: opts.VersionID, DryRun: opts.DryRun}
	if opts.DryRun {
		rmEmit(res, opts, human, out)
		return []objectstorage.DeleteResult{res}, nil
	}
	err := sess.Client.RemoveObject(ctx, sess.Bucket.BucketName, key, minio.RemoveObjectOptions{
		VersionID:        opts.VersionID,
		GovernanceBypass: opts.BypassGovernance,
	})
	if err != nil {
		return nil, sess.humanize(err)
	}
	res.Deleted = true
	rmEmit(res, opts, human, out)
	return []objectstorage.DeleteResult{res}, nil
}

// rmEmit prints one result in human mode, honouring --quiet and
// --only-show-errors (both silence successes only, like aws). Failures
// always go to stderr; structured mode prints nothing here.
func rmEmit(res objectstorage.DeleteResult, opts rmOptions, human bool, out io.Writer) {
	if res.Error != "" {
		if human {
			fmt.Fprintln(os.Stderr, res.HumanLine())
		}
		return
	}
	if !human || opts.Quiet || opts.OnlyShowErrors {
		return
	}
	fmt.Fprintln(out, res.HumanLine())
}

// rmPlan enumerates the objects under prefix that pass the filters.
func rmPlan(ctx context.Context, sess *Session, prefix string, opts rmOptions) ([]minio.ObjectInfo, int64, error) {
	var (
		objects []minio.ObjectInfo
		total   int64
	)
	for info := range sess.Client.ListObjects(ctx, sess.Bucket.BucketName, minio.ListObjectsOptions{
		Prefix:       prefix,
		Recursive:    true,
		WithVersions: opts.Versions,
	}) {
		if info.Err != nil {
			return nil, 0, sess.humanize(info.Err)
		}
		if info.Key == "" {
			continue
		}
		// Filters see the key relative to the prefix, like aws.
		rel := strings.TrimPrefix(info.Key, prefix)
		if !opts.Filters.Include(rel) {
			continue
		}
		objects = append(objects, info)
		total += info.Size
	}
	return objects, total, nil
}

// rmRecursive is `rm --recursive`: lock checks, enumeration, --max-delete,
// confirmation, batched deletion and the partial-failure summary.
func rmRecursive(ctx context.Context, sess *Session, prefix string, opts rmOptions, human bool, out io.Writer, confirm func(string) error) ([]objectstorage.DeleteResult, error) {
	if err := rmCheckObjectLock(sess.Bucket, opts.BypassGovernance); err != nil {
		return nil, err
	}
	label := rmBucketLabel(sess.Bucket)
	target := "s3://" + label
	if prefix != "" {
		target += "/" + prefix
	}

	objects, total, err := rmPlan(ctx, sess, prefix, opts)
	if err != nil {
		return nil, err
	}
	what := "objects"
	if opts.Versions {
		what = "object versions"
	}
	if opts.MaxDelete > 0 && len(objects) > opts.MaxDelete {
		return nil, exitcode.Errorf(exitcode.Refused, "%d %s match under %s but --max-delete is %d; refusing to delete anything", len(objects), what, target, opts.MaxDelete)
	}

	if opts.DryRun {
		rows := make([]objectstorage.DeleteResult, 0, len(objects))
		for _, o := range objects {
			res := objectstorage.DeleteResult{Bucket: label, Key: o.Key, VersionID: o.VersionID, DryRun: true}
			rmEmit(res, opts, human, out)
			rows = append(rows, res)
		}
		objectstorage.Hintf("would delete %d %s (%s)", len(objects), what, objectstorage.HumanSize(total))
		return rows, nil
	}
	if len(objects) == 0 {
		return nil, nil
	}

	// target already ends with "/" for a prefix.
	question := fmt.Sprintf("Delete %d %s under %s?", len(objects), what, target)
	if prefix == "" {
		question = fmt.Sprintf("Delete all %d %s in %s (%s)?", len(objects), what, target, rmBucketDetails(sess.Bucket))
	}
	if err := confirm(question); err != nil {
		return nil, err
	}

	var (
		rows            []objectstorage.DeleteResult
		deleted, failed int
		stopped         bool
	)
	for start := 0; start < len(objects) && !stopped; start += rmBatchSize {
		end := start + rmBatchSize
		if end > len(objects) {
			end = len(objects)
		}
		results := rmDeleteBatch(ctx, sess, objects[start:end], opts.BypassGovernance, label)
		for _, res := range results {
			rmEmit(res, opts, human, out)
			rows = append(rows, res)
			if res.Error == "" {
				deleted++
				continue
			}
			failed++
			if rmIsRetentionError(res.Error) {
				stopped = true
			}
		}
		if stopped {
			remaining := len(objects) - end
			if remaining > 0 {
				objectstorage.Warnf("stopped after a retention error; %d %s were not attempted", remaining, what)
			}
		}
		if ctx.Err() != nil {
			return rows, objectstorage.Humanize(ctx.Err(), sess.Bucket, &sess.Cred)
		}
	}
	if failed > 0 {
		return rows, exitcode.Errorf(exitcode.Partial, "%d deleted, %d failed", deleted, failed)
	}
	return rows, nil
}

// rmBucketDetails renders the identity shown when the whole bucket is at
// stake: id, backend name and project.
func rmBucketDetails(b *objectstorage.Bucket) string {
	parts := []string{}
	if b.ID != "" {
		parts = append(parts, b.ID)
	}
	if b.BucketName != "" && b.BucketName != b.Name {
		parts = append(parts, "backend name "+b.BucketName)
	}
	if p := b.ProjectRef(); p != "" {
		parts = append(parts, "project "+p)
	}
	if len(parts) == 0 {
		return b.BucketName
	}
	return strings.Join(parts, ", ")
}

// rmDeleteBatch deletes up to rmBatchSize objects with one multi-delete call
// and returns a result per object, in input order. A request-level failure
// (the whole POST rejected) marks every object of the batch as failed.
func rmDeleteBatch(ctx context.Context, sess *Session, batch []minio.ObjectInfo, bypass bool, label string) []objectstorage.DeleteResult {
	objectsCh := make(chan minio.ObjectInfo, len(batch))
	for _, o := range batch {
		objectsCh <- o
	}
	close(objectsCh)

	failed := map[string]error{}
	var batchErr error
	for e := range sess.Client.RemoveObjects(ctx, sess.Bucket.BucketName, objectsCh, minio.RemoveObjectsOptions{GovernanceBypass: bypass}) {
		if e.ObjectName == "" {
			batchErr = e.Err
			continue
		}
		failed[rmObjectID(e.ObjectName, e.VersionID)] = e.Err
	}

	results := make([]objectstorage.DeleteResult, 0, len(batch))
	for _, o := range batch {
		res := objectstorage.DeleteResult{Bucket: label, Key: o.Key, VersionID: o.VersionID}
		err, ok := failed[rmObjectID(o.Key, o.VersionID)]
		if !ok {
			err, ok = failed[rmObjectID(o.Key, "")]
		}
		switch {
		case ok:
			res.Error = rmErrorText(err)
		case batchErr != nil:
			res.Error = rmErrorText(batchErr)
		default:
			res.Deleted = true
		}
		results = append(results, res)
	}
	return results
}

func rmObjectID(key, versionID string) string { return key + "\x00" + versionID }

// rmErrorText renders a per-object failure compactly (code: message).
func rmErrorText(err error) string {
	if err == nil {
		return "unknown error"
	}
	if resp := minio.ToErrorResponse(err); resp.Code != "" {
		if resp.Message != "" {
			return resp.Code + ": " + resp.Message
		}
		return resp.Code
	}
	return objectstorage.Redact(err.Error())
}

// rmIsRetentionError reports whether a per-object failure text denotes an
// object lock / retention refusal, after which the command stops.
func rmIsRetentionError(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "objectlocked") ||
		strings.Contains(t, "retention") ||
		strings.Contains(t, "object lock") ||
		strings.Contains(t, "governance")
}

// Compile-time check that DeleteResult renders through the shared renderer.
var _ renderer.ResponseData = objectstorage.DeleteResult{}
