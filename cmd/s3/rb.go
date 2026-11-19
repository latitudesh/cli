package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
	cobra "github.com/spf13/cobra"
)

// Flag names of rb.
const (
	flagRbForce     = "force"
	flagRbVersions  = "versions"
	flagRbBypass    = "bypass-governance-retention"
	flagRbMaxDelete = "max-delete"
)

// rbDeleteBatch caps the objects sent per multi-delete request.
const rbDeleteBatch = 1000

// rbDeleteBucketAPI deletes the bucket record through the Latitude API. It is
// a variable so tests can stub the control plane.
var rbDeleteBucketAPI = func(ctx context.Context, b *objectstorage.Bucket) error {
	client := apiClient()
	_, err := client.ObjectStorage.DeleteStorageBuckets(ctx, b.ID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		return rbHumanizeAPIDelete(objectstorage.Humanize(err, nil, nil), b)
	}
	return nil
}

// rbHumanizeAPIDelete adds the --force hint to "not empty" refusals from the
// API (409/422 mentioning not empty, or an S3 BucketNotEmpty passthrough).
func rbHumanizeAPIDelete(err error, b *objectstorage.Bucket) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not empty") || strings.Contains(msg, "bucketnotempty") {
		return exitcode.Errorf(exitcode.Refused, "bucket %s is not empty; re-run with --force to delete its objects first (add --versions for versioned buckets)", b.Display())
	}
	return err
}

// rbOptions are the inputs of the --force flow.
type rbOptions struct {
	Versions  bool
	Bypass    bool
	Yes       bool
	MaxDelete int64
	DryRun    bool
	Human     bool
}

// NewRbCmd builds `lsh s3 delete-bucket s3://bucket`.
func NewRbCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "delete-bucket s3://bucket",
		Aliases: []string{"rb"},
		GroupID: groupBuckets,
		Short:   "Delete a bucket (alias: rb)",
		Long: `Delete a bucket through the Latitude API.

Without --force the bucket must be empty; the API refuses otherwise (exit 7).
With --force the CLI lists the objects, asks for confirmation (or --yes),
deletes them and then removes the bucket. Versioned buckets also need
--versions so every object version and delete marker is removed. Buckets with COMPLIANCE object lock cannot be
force-deleted; GOVERNANCE retention needs --bypass-governance-retention.

--force never skips the confirmation: pass --yes in scripts. Without a TTY and
without --yes the command fails fast with exit 7.`,
		Example: `  lsh s3 delete-bucket s3://backups
  lsh s3 delete-bucket s3://backups --force
  lsh s3 delete-bucket s3://backups --force --versions --yes
  lsh s3 delete-bucket s3://backups --force --max-delete 100 --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runRb,
	})
	addProjectFlag(cmd, true, "project ID or slug to disambiguate the bucket name")
	addYesFlag(cmd)
	addBucketFilterFlags(cmd)
	cmd.Flags().Bool(flagRbForce, false, "delete all objects in the bucket first")
	cmd.Flags().Bool(flagRbVersions, false, "with --force: also delete every object version and delete marker")
	cmd.Flags().Bool(flagRbBypass, false, "with --force: bypass GOVERNANCE object lock retention")
	cmd.Flags().Int64(flagRbMaxDelete, 0, "with --force: refuse when more than N objects would be deleted (0 = no limit)")
	return cmd
}

func runRb(cmd *cobra.Command, args []string) error {
	ref, err := objectstorage.ParseBucketOnly(args[0])
	if err != nil {
		return printErr(err)
	}
	if endpointOverride(cmd) != "" {
		return printErr(exitcode.Errorf(exitcode.Usage, "rb deletes the bucket through the Latitude API and cannot work with --endpoint-url / %s; unset it and address the bucket by name or bkt_ ID", objectstorage.EnvEndpointURL))
	}
	f := cmd.Flags()
	force, _ := f.GetBool(flagRbForce)
	yes, _ := f.GetBool(flagYes)
	o := rbOptions{Yes: yes, DryRun: dryRun(), Human: isHuman()}
	o.Versions, _ = f.GetBool(flagRbVersions)
	o.Bypass, _ = f.GetBool(flagRbBypass)
	o.MaxDelete, _ = f.GetInt64(flagRbMaxDelete)
	if o.MaxDelete < 0 {
		return printErr(exitcode.Errorf(exitcode.Usage, "--max-delete must be zero or a positive number"))
	}
	if !force && (o.Versions || o.Bypass || o.MaxDelete > 0) {
		return printErr(exitcode.Errorf(exitcode.Usage, "--versions, --bypass-governance-retention and --max-delete only apply with --force"))
	}

	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	b, err := resolveBucket(ctx, cmd, ref.Bucket)
	if err != nil {
		return printErr(err)
	}

	if !force {
		row := objectstorage.DeleteResult{Bucket: b.Name, DryRun: o.DryRun}
		if !o.DryRun {
			if err := rbDeleteBucketAPI(ctx, b); err != nil {
				return printErr(err)
			}
			row.Deleted = true
		}
		rbEmitResults(o.Human, os.Stdout, []objectstorage.DeleteResult{row})
		return nil
	}

	sess, err := openResolved(cmd, b, true)
	if err != nil {
		return printErr(err)
	}
	rows, err := forceRemoveBucket(ctx, cmd, sess, o, os.Stdout)
	if !o.Human && len(rows) > 0 {
		render(objectstorage.AsResponseData(rows))
	}
	if err != nil {
		return printErr(err)
	}
	return nil
}

// forceRemoveBucket empties the bucket and deletes it (or prints the plan in
// dry-run mode). In human mode the aws-style lines are written to w as they
// happen; otherwise the rows are returned for the caller to render. The
// returned rows are also populated on partial failure so structured output
// still lists what was deleted.
func forceRemoveBucket(ctx context.Context, cmd *cobra.Command, sess *Session, o rbOptions, w io.Writer) ([]objectstorage.DeleteResult, error) {
	b := sess.Bucket
	if b.Locking && strings.EqualFold(b.RetentionMode, string(operations.RetentionModeCompliance)) {
		return nil, exitcode.Errorf(exitcode.Refused, "bucket %s has COMPLIANCE object lock: its objects cannot be deleted until their retention expires, so the bucket cannot be force-deleted", b.Display())
	}
	if b.Versioning && !o.Versions {
		return nil, exitcode.Errorf(exitcode.Refused, "bucket %s is versioned; add --versions to delete all object versions and delete markers as well", b.Display())
	}
	if b.Locking && !o.Bypass && !o.DryRun {
		objectstorage.Hintf("note: bucket %s has %s object lock; objects still under retention will fail to delete unless you pass --bypass-governance-retention", b.Display(), strings.ToUpper(b.RetentionMode))
	}

	objects, err := rbListAllObjects(ctx, sess.Client, b.BucketName, o.Versions)
	if err != nil {
		return nil, sess.humanize(err)
	}
	if o.MaxDelete > 0 && int64(len(objects)) > o.MaxDelete {
		return nil, exitcode.Errorf(exitcode.Refused, "bucket %s holds %s %s, more than --max-delete %d; nothing was deleted", b.Display(), rbCommaInt(int64(len(objects))), rbNoun(o.Versions, len(objects)), o.MaxDelete)
	}

	if o.DryRun {
		rows := make([]objectstorage.DeleteResult, 0, len(objects)+1)
		for _, obj := range objects {
			rows = append(rows, rbDeleteRow(b.Name, obj, o.Versions, true))
		}
		rows = append(rows, objectstorage.DeleteResult{Bucket: b.Name, DryRun: true})
		if o.Human {
			rbEmitResults(true, w, rows)
		}
		return rows, nil
	}

	question := fmt.Sprintf("Delete bucket %s (%s, project %s, backend %s) and its %s %s?",
		b.Name, b.ID, b.ProjectRef(), b.BucketName, rbCommaInt(int64(len(objects))), rbNoun(o.Versions, len(objects)))
	if len(objects) == 0 {
		question = fmt.Sprintf("Delete bucket %s (%s, project %s, backend %s)? It is empty.", b.Name, b.ID, b.ProjectRef(), b.BucketName)
	}
	if err := objectstorage.ConfirmOrRefuse(cmd, o.Yes, question); err != nil {
		return nil, err
	}

	rows, deleted, failed := rbDeleteObjects(ctx, sess, objects, o, w)
	// An interrupt aborts the batches mid-flight and inflates failed; report
	// the signal instead of a partial run (same contract as rm).
	if ctx.Err() != nil {
		return rows, objectstorage.Humanize(ctx.Err(), sess.Bucket, &sess.Cred)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d deleted, %d failed; bucket %s was not removed\n", deleted, failed, b.Display())
		return rows, exitcode.Errorf(exitcode.Partial, "could not delete every object of bucket %s (%d deleted, %d failed)", b.Display(), deleted, failed)
	}

	if err := rbDeleteBucketAPI(ctx, b); err != nil {
		return rows, err
	}
	final := objectstorage.DeleteResult{Bucket: b.Name, Deleted: true}
	rows = append(rows, final)
	if o.Human {
		fmt.Fprintln(w, final.HumanLine())
	}
	return rows, nil
}

// rbListAllObjects enumerates every object (or every version and delete marker
// when withVersions is set) in the bucket.
func rbListAllObjects(ctx context.Context, client *minio.Client, bucket string, withVersions bool) ([]minio.ObjectInfo, error) {
	var out []minio.ObjectInfo
	for info := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true, WithVersions: withVersions}) {
		if info.Err != nil {
			return nil, info.Err
		}
		out = append(out, info)
	}
	return out, nil
}

// rbDeleteObjects removes the objects in batches. It stops at the first batch
// with a failure. Human mode prints one line per object to w.
func rbDeleteObjects(ctx context.Context, sess *Session, objects []minio.ObjectInfo, o rbOptions, w io.Writer) (rows []objectstorage.DeleteResult, deleted, failed int) {
	b := sess.Bucket
	for start := 0; start < len(objects); start += rbDeleteBatch {
		end := start + rbDeleteBatch
		if end > len(objects) {
			end = len(objects)
		}
		batch := objects[start:end]
		if ctx.Err() != nil {
			return rows, deleted, failed
		}

		ch := make(chan minio.ObjectInfo, len(batch))
		for _, obj := range batch {
			item := minio.ObjectInfo{Key: obj.Key}
			if o.Versions && obj.VersionID != "" && obj.VersionID != "null" {
				item.VersionID = obj.VersionID
			}
			ch <- item
		}
		close(ch)

		results := sess.Client.RemoveObjectsWithResult(ctx, b.BucketName, ch, minio.RemoveObjectsOptions{GovernanceBypass: o.Bypass})
		seen := map[string]bool{}
		for res := range results {
			row := objectstorage.DeleteResult{Bucket: b.Name, Key: res.ObjectName}
			if o.Versions {
				row.VersionID = res.ObjectVersionID
			}
			if res.Err != nil {
				row.Error = sess.humanize(res.Err).Error()
				failed++
			} else {
				row.Deleted = true
				deleted++
			}
			seen[row.Key+"\x00"+row.VersionID] = true
			rows = append(rows, row)
			if o.Human {
				fmt.Fprintln(w, row.HumanLine())
			}
		}
		// Backends in quiet mode report nothing for successes: account for the
		// objects of a batch that produced no result as deleted.
		for _, obj := range batch {
			vid := ""
			if o.Versions && obj.VersionID != "" && obj.VersionID != "null" {
				vid = obj.VersionID
			}
			if seen[obj.Key+"\x00"+vid] {
				continue
			}
			row := objectstorage.DeleteResult{Bucket: b.Name, Key: obj.Key, VersionID: vid, Deleted: true}
			deleted++
			rows = append(rows, row)
			if o.Human {
				fmt.Fprintln(w, row.HumanLine())
			}
		}
		if failed > 0 {
			return rows, deleted, failed
		}
	}
	return rows, deleted, failed
}

// rbDeleteRow builds the row for one listed object.
func rbDeleteRow(bucket string, obj minio.ObjectInfo, withVersions, dry bool) objectstorage.DeleteResult {
	row := objectstorage.DeleteResult{Bucket: bucket, Key: obj.Key, DryRun: dry}
	if withVersions && obj.VersionID != "" && obj.VersionID != "null" {
		row.VersionID = obj.VersionID
	}
	return row
}

// rbEmitResults prints human lines to w, or renders the rows.
func rbEmitResults(human bool, w io.Writer, rows []objectstorage.DeleteResult) {
	if human {
		for _, r := range rows {
			fmt.Fprintln(w, r.HumanLine())
		}
		return
	}
	out := make([]renderer.ResponseData, 0, len(rows))
	for _, r := range rows {
		out = append(out, r)
	}
	render(out)
}

func rbNoun(versions bool, n int) string {
	switch {
	case versions && n == 1:
		return "object version"
	case versions:
		return "object versions"
	case n == 1:
		return "object"
	default:
		return "objects"
	}
}

// rbCommaInt renders 1204 as "1,204".
func rbCommaInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var sb strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(r)
	}
	if neg {
		return "-" + sb.String()
	}
	return sb.String()
}
