package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/minio/minio-go/v7"
	cobra "github.com/spf13/cobra"
)

const flagStatVersionID = "version-id"

// NewStatCmd builds `lsh s3 get s3://bucket[/key]`.
func NewStatCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:        "get s3://bucket[/key]",
		Aliases:    []string{"stat", "describe", "head"},
		GroupID:    groupBuckets,
		SuggestFor: []string{"show", "info"},
		Short:      "Show a bucket or an object (alias: stat)",
		Long: `Show a bucket or an object.

For a bucket (s3://bucket) the details come from the Latitude API: id, names,
endpoint, site, signing region, class, versioning, object lock and which saved
access keys cover it. For an object (s3://bucket/key) the CLI issues a HEAD
request to the S3 endpoint: size, ETag, content type, last modified, version
and user metadata.`,
		Example: `  lsh s3 get s3://backups
  lsh s3 get s3://backups/2026/09/dump.sql
  lsh s3 get s3://backups/2026/09/dump.sql --version-id 3HL4kqtJlcpXroDTDmJ
  lsh s3 get s3://backups -o json`,
		Args: cobra.ExactArgs(1),
		RunE: runStat,
	})
	addProjectFlag(cmd, true, "project ID or slug to disambiguate the bucket name")
	addBucketFilterFlags(cmd)
	cmd.Flags().String(flagStatVersionID, "", "inspect this object version")
	rejectRegionFlag(cmd)
	return cmd
}

func runStat(cmd *cobra.Command, args []string) error {
	ref, err := objectstorage.ParseRemote(args[0])
	if err != nil {
		return printErr(err)
	}
	versionID, _ := cmd.Flags().GetString(flagStatVersionID)
	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	if ref.Key == "" {
		if versionID != "" {
			return printErr(objectstorage.ErrUsagef("--version-id only applies to objects (s3://bucket/key)"))
		}
		b, err := resolveBucket(ctx, cmd, ref.Bucket)
		if err != nil {
			return printErr(err)
		}
		if err := newResolver(cmd).FillSite(ctx, b); err != nil && lsh.Debug {
			fmt.Fprintf(os.Stderr, "[s3] could not fill site for %s: %v\n", b.Display(), err)
		}
		if !isHuman() {
			render([]renderer.ResponseData{NewBucketRow(b)})
			return nil
		}
		writeBucketStat(os.Stdout, b, savedKeysCovering(cmd, b))
		return nil
	}

	if strings.HasSuffix(ref.Key, "/") {
		return printErr(objectstorage.ErrUsagef("key %q ends with '/'; stat inspects a single object (use 'lsh s3 list %s' to list a prefix)", ref.Key, ref))
	}
	sess, err := openBucket(ctx, cmd, ref.Bucket, false)
	if err != nil {
		return printErr(err)
	}
	obj, err := statObject(ctx, sess, ref.Key, versionID)
	if err != nil {
		return printErr(err)
	}
	if !isHuman() {
		render([]renderer.ResponseData{obj})
		return nil
	}
	writeObjectStat(os.Stdout, obj)
	return nil
}

// statObject issues the HEAD request and converts the result.
func statObject(ctx context.Context, sess *Session, key, versionID string) (objectstorage.Object, error) {
	info, err := sess.Client.StatObject(ctx, sess.Bucket.BucketName, key, minio.StatObjectOptions{VersionID: versionID})
	if err != nil {
		return objectstorage.Object{}, sess.humanize(err)
	}
	// A HEAD response never says whether the version is the latest, so the
	// listing-only IsLatest flag stays unset; the version ID is still copied
	// whenever the backend returns one.
	obj := objectstorage.ObjectFromInfo(info, false)
	if obj.Key == "" {
		obj.Key = key
	}
	obj.Metadata = statUserMetadata(info)
	return obj, nil
}

// statUserMetadata collects x-amz-meta-* values from a HEAD response. minio fills
// UserMetadata with the prefix stripped; the raw headers are consulted too for
// backends that only surface them there.
func statUserMetadata(info minio.ObjectInfo) map[string]string {
	out := map[string]string{}
	for k, v := range info.UserMetadata {
		lk := strings.ToLower(strings.TrimPrefix(strings.ToLower(k), "x-amz-meta-"))
		if lk == "" || v == "" {
			continue
		}
		out[lk] = v
	}
	for k, v := range info.Metadata {
		lk := strings.ToLower(k)
		if !strings.HasPrefix(lk, "x-amz-meta-") || len(v) == 0 {
			continue
		}
		name := strings.TrimPrefix(lk, "x-amz-meta-")
		if _, dup := out[name]; !dup && name != "" {
			out[name] = v[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// kv is one "Label: value" line of the human output.
type statKV struct {
	label string
	value string
}

// statWriteKV prints aligned "Label: value" lines.
func statWriteKV(w io.Writer, pairs []statKV) {
	width := 0
	for _, p := range pairs {
		if len(p.label) > width {
			width = len(p.label)
		}
	}
	for _, p := range pairs {
		fmt.Fprintf(w, "%-*s %s\n", width+1, p.label+":", p.value)
	}
}

// writeBucketStat renders the bucket details for humans.
func writeBucketStat(w io.Writer, b *objectstorage.Bucket, keys []statCoveringKey) {
	pairs := []statKV{}
	if !b.EndpointOverride {
		pairs = append(pairs,
			statKV{"ID", b.ID},
			statKV{"Name", b.Name},
		)
	}
	pairs = append(pairs, statKV{"Bucket name (backend)", b.BucketName})
	if !b.EndpointOverride {
		project := b.ProjectRef()
		if b.ProjectName != "" && b.ProjectName != project {
			project = fmt.Sprintf("%s (%s)", project, b.ProjectName)
		}
		pairs = append(pairs,
			statKV{"Project", orEmptyCell(project)},
			statKV{"Class", orEmptyCell(b.StorageClass)},
			statKV{"Site", orEmptyCell(firstNonEmptyStr(b.Site, b.City))},
		)
	}
	pairs = append(pairs,
		statKV{"Endpoint", orEmptyCell(b.Endpoint)},
		statKV{"Signing region", orEmptyCell(b.SigningRegion)},
	)
	if !b.EndpointOverride {
		created := ""
		if b.CreatedAt != nil {
			created = objectstorage.FormatTime(*b.CreatedAt)
		}
		pairs = append(pairs,
			statKV{"Versioning", yesNo(b.Versioning)},
			statKV{"Locking", lockingLabel(b)},
			statKV{"Source", orEmptyCell(b.Source)},
			statKV{"Created", orEmptyCell(created)},
		)
	}
	statWriteKV(w, pairs)
	if b.EndpointOverride {
		return
	}
	fmt.Fprintln(w, "Access keys covering this bucket:")
	if len(keys) == 0 {
		fmt.Fprintln(w, "  (none saved)")
		return
	}
	for _, k := range keys {
		fmt.Fprintf(w, "  %s (%s)\n", k.Name, k.Permission)
	}
}

// writeObjectStat renders the object details for humans.
func writeObjectStat(w io.Writer, o objectstorage.Object) {
	pairs := []statKV{
		{"Key", o.Key},
		{"Size", fmt.Sprintf("%d (%s)", o.Size, objectstorage.HumanSize(o.Size))},
		{"Last modified", orEmptyCell(objectstorage.FormatTime(o.LastModified))},
		{"ETag", orEmptyCell(o.ETag)},
		{"Content type", orEmptyCell(o.ContentType)},
	}
	if o.VersionID != "" {
		pairs = append(pairs, statKV{"Version ID", o.VersionID})
	}
	if o.StorageClass != "" {
		pairs = append(pairs, statKV{"Storage class", o.StorageClass})
	}
	if o.IsDeleteMarker {
		pairs = append(pairs, statKV{"Delete marker", "yes"})
	}
	if len(o.Metadata) > 0 {
		names := make([]string, 0, len(o.Metadata))
		for k := range o.Metadata {
			names = append(names, k)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, k := range names {
			parts = append(parts, k+"="+o.Metadata[k])
		}
		pairs = append(pairs, statKV{"Metadata", strings.Join(parts, " ")})
	}
	statWriteKV(w, pairs)
}

// statCoveringKey is a saved key that grants access to the bucket.
type statCoveringKey struct {
	Name       string
	Permission string
}

// savedKeysCovering lists the saved keys of the active profile that cover b,
// sorted by name. Profile problems are not errors for stat: the list is
// simply empty.
func savedKeysCovering(cmd *cobra.Command, b *objectstorage.Bucket) []statCoveringKey {
	_, _, profile, err := objectstorage.ActiveProfile(profileFlag(cmd))
	if err != nil {
		return nil
	}
	return statCoveringKeys(profile.ObjectStorageKeys(), b)
}

// statCoveringKeys applies the class/site/project compatibility rules and
// StoredAccessKey.Covers to the saved keys.
func statCoveringKeys(keys map[string]config.StoredAccessKey, b *objectstorage.Bucket) []statCoveringKey {
	var out []statCoveringKey
	for name, k := range keys {
		if k.Scope == config.ScopeUnknown {
			continue
		}
		if k.StorageClass != "" && b.StorageClass != "" && k.StorageClass != b.StorageClass {
			continue
		}
		if k.Site != "" && b.Site != "" && !strings.EqualFold(k.Site, b.Site) {
			continue
		}
		if k.ProjectID != "" && b.ProjectID != "" && k.ProjectID != b.ProjectID {
			continue
		}
		if !k.Covers(b.ID, false) {
			continue
		}
		perm := k.Permission(b.ID)
		if k.Scope == config.ScopeFullAccess {
			perm = "fullaccess"
		}
		out = append(out, statCoveringKey{Name: name, Permission: perm})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
