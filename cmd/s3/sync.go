package s3

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/spf13/cobra"
)

// syncOptions are the flags specific to sync.
type syncOptions struct {
	// Delete removes destination entries that are absent from the source.
	Delete bool
	// SizeOnly compares only sizes.
	SizeOnly bool
	// ExactTimestamps makes same-sized downloads skip only when the
	// timestamps match exactly.
	ExactTimestamps bool
}

// NewSyncCmd builds `lsh s3 sync <source> <destination>`.
func NewSyncCmd() *cobra.Command {
	var f transferFlags
	var so syncOptions
	cmd := newCmd(&cobra.Command{
		Use:     "sync <source> <destination>",
		GroupID: groupObjects,
		Short:   "Synchronize directories and prefixes",
		Long: `Recursively copy new and changed files between a local directory and a
prefix, or between two prefixes on the same endpoint.

An entry is transferred when it is missing at the destination, its size
differs, or the source is newer than the destination. --size-only ignores
timestamps; --exact-timestamps (S3 -> local) skips same-sized files only when
the timestamps match exactly. ETags are never compared. --delete removes
destination entries that no longer exist at the source; --exclude/--include
apply to paths relative to the source (and to the destination for --delete).
Deletions run after the copies and are skipped entirely if any copy failed, so
a partial run never leaves the destination without both versions; re-run the
sync to apply them. They also never leave the destination tree: a candidate
reached through a directory symlink is reported and kept.

Output lines use upload:/download:/copy:/delete:. --dry-run (or --dry-run)
prints the plan without writing or deleting anything.`,
		Example: `  lsh s3 sync ./site s3://www
  lsh s3 sync s3://backups/2026/ ./backups/2026/
  lsh s3 sync ./logs s3://logs/host-1/ --exclude "*" --include "*.log" --delete --dry-run
  lsh s3 sync s3://backups/ s3://archive/backups/ --size-only`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncCommand(cmd, args, &f, so)
		},
	})
	addTransferFlags(cmd, &f, false)
	cmd.Flags().BoolVar(&so.Delete, "delete", false, "delete destination entries that are absent from the source (skipped when a copy fails)")
	cmd.Flags().BoolVar(&so.SizeOnly, "size-only", false, "compare by size only, ignoring timestamps")
	cmd.Flags().BoolVar(&so.ExactTimestamps, "exact-timestamps", false, "S3 -> local: skip same-sized files only when the timestamps match exactly")
	return cmd
}

// runSyncCommand is the RunE body of sync.
func runSyncCommand(cmd *cobra.Command, args []string, f *transferFlags, so syncOptions) error {
	srcRef, dstRef, err := classifyOperands(args[0], args[1], false, true)
	if err != nil {
		return printErr(err)
	}
	opts, err := buildTransferOptions(cmd, f)
	if err != nil {
		return printErr(err)
	}
	opts.Recursive = true

	ctx, stop := objectstorage.SignalContext(context.Background())
	defer stop()

	src, dst, err := openEndpoints(ctx, cmd, srcRef, dstRef, false)
	if err != nil {
		return printErr(err)
	}
	plan, err := buildSyncPlan(ctx, src, dst, opts, so)
	if err != nil {
		return printErr(err)
	}
	results, err := runTransfers(ctx, plan, opts, os.Stdout, os.Stderr)
	return finishTransfer(plan, opts, results, err)
}

// buildSyncPlan enumerates both sides and decides what to transfer and, with
// --delete, what to remove. Only read-only calls are made.
func buildSyncPlan(ctx context.Context, src, dst endpoint, opts *transferOptions, so syncOptions) (*transferPlan, error) {
	plan := &transferPlan{Src: src, Dst: dst}
	if err := checkCrossEndpoint(src, dst); err != nil {
		return nil, err
	}
	if src.local() && dst.local() {
		return nil, objectstorage.ErrUsagef("both operands are local paths; at least one must be an s3:// location")
	}

	srcEntries, err := enumerate(ctx, src, opts.FollowSymlinks)
	if err != nil {
		return nil, err
	}
	dstEntries, err := enumerateDestination(ctx, dst, opts.FollowSymlinks)
	if err != nil {
		return nil, err
	}
	dstByRel := make(map[string]entry, len(dstEntries))
	for _, e := range dstEntries {
		dstByRel[e.Rel] = e
	}

	op := opFor(src, dst, false)
	download := src.remote() && dst.local()
	dstPrefix := ""
	if dst.remote() {
		dstPrefix = objectstorage.NormalizePrefix(dst.Ref.Key)
	}
	inSource := make(map[string]bool, len(srcEntries))
	for _, e := range srcEntries {
		if !opts.Filters.Include(e.Rel) {
			continue
		}
		inSource[e.Rel] = true
		d, exists := dstByRel[e.Rel]
		if !syncNeedsTransfer(e, d, exists, so, download) {
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

	if so.Delete {
		// A local destination may contain directory symlinks, which the
		// enumeration follows by default. Deleting through one of them would
		// remove files outside the tree the user pointed at, so a candidate
		// whose parent directory resolves outside the destination root is
		// skipped with a warning. Removing a link that itself lives inside the
		// root stays allowed: only the link goes away, not its target.
		root := ""
		if dst.local() {
			if resolved, err := filepath.EvalSymlinks(dst.Ref.Raw); err == nil {
				root = resolved
			}
		}
		for _, d := range dstEntries {
			if inSource[d.Rel] || !opts.Filters.Include(d.Rel) {
				continue
			}
			if root != "" && !deleteInsideRoot(root, d.Path) {
				objectstorage.Warnf("not deleting %s: it resolves outside %s through a symbolic link; pass --no-follow-symlinks to leave links out of the sync", displayLocal(d.Path), displayLocal(dst.Ref.Raw))
				continue
			}
			del := deleteItem{Path: d.Path, Key: d.Key}
			if dst.remote() {
				del.Display = dst.remoteURI(d.Key)
			} else {
				del.Display = displayLocal(d.Path)
			}
			plan.Deletes = append(plan.Deletes, del)
		}
	}
	return plan, nil
}

// deleteInsideRoot reports whether removing path only affects the destination
// tree: its parent directory, with every symbolic link resolved, must be root
// or below it. root is expected to be resolved already.
func deleteInsideRoot(root, path string) bool {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, parent)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// enumerateDestination lists the destination; a local directory that does
// not exist yet is simply empty.
func enumerateDestination(ctx context.Context, dst endpoint, followSymlinks bool) ([]entry, error) {
	if dst.remote() {
		return enumerate(ctx, dst, followSymlinks)
	}
	st, err := os.Stat(dst.Ref.Raw)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, localErr(err)
	case !st.IsDir():
		return nil, objectstorage.ErrUsagef("%s is not a directory", dst.Ref.Raw)
	}
	return enumerate(ctx, dst, followSymlinks)
}

// syncNeedsTransfer applies the comparison rules. Timestamps are compared at
// second precision because S3 does not keep sub-second modification times.
func syncNeedsTransfer(src, dst entry, exists bool, so syncOptions, download bool) bool {
	if !exists {
		return true
	}
	if src.Size != dst.Size {
		return true
	}
	if so.SizeOnly {
		return false
	}
	s, d := src.ModTime.Truncate(time.Second), dst.ModTime.Truncate(time.Second)
	if download && so.ExactTimestamps {
		return !s.Equal(d)
	}
	return s.After(d)
}
