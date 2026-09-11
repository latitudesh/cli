package s3

import (
	"context"

	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/spf13/cobra"
)

// newLifecycleGetCmd builds `lsh s3 lifecycle get s3://bucket <rule-id|name>`.
func newLifecycleGetCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "get <s3://bucket> <rule-id|name>",
		Aliases: []string{"show", "describe"},
		Short:   "Show one lifecycle rule",
		Long: `Show one lifecycle rule of a bucket, addressed by id (lifecycle_...) or by
name. A name shared by several rules is rejected (exit 2): use the id.`,
		Example: `  lsh s3 lifecycle get s3://logs expire-7d-tmp
  lsh s3 lifecycle get s3://logs lifecycle_9x4kQ -o json`,
		Args: cobra.ExactArgs(2),
		RunE: runLifecycleGet,
	})
	addProjectFlag(cmd, true, "project of the bucket (disambiguates names shared across projects)")
	addBucketFilterFlags(cmd)
	return cmd
}

func runLifecycleGet(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	lc, err := openLifecycleAPI(ctx, cmd, args[0])
	if err != nil {
		return printErr(err)
	}
	rule, err := lc.get(ctx, args[1])
	if err != nil {
		return printErr(err)
	}
	render([]renderer.ResponseData{LifecycleRule{LifecycleRuleData: *rule}})
	return nil
}
