package s3

import (
	"context"

	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/spf13/cobra"
)

// newLifecycleListCmd builds `lsh s3 lifecycle list s3://bucket`.
func newLifecycleListCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "list <s3://bucket>",
		Aliases: []string{"ls"},
		Short:   "List the lifecycle rules of a bucket",
		Long: `List the lifecycle rules of a bucket.

A bucket without rules prints an empty list and exits 0.`,
		Example: `  lsh s3 lifecycle list s3://logs
  lsh s3 lifecycle ls logs -o json`,
		Args: cobra.ExactArgs(1),
		RunE: runLifecycleList,
	})
	addProjectFlag(cmd, true, "project of the bucket (disambiguates names shared across projects)")
	addBucketFilterFlags(cmd)
	return cmd
}

func runLifecycleList(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	lc, err := openLifecycleAPI(ctx, cmd, args[0])
	if err != nil {
		return printErr(err)
	}
	rules, err := lc.list(ctx)
	if err != nil {
		return printErr(err)
	}
	if len(rules) == 0 && isHuman() {
		// Empty is a normal outcome (exit 0): say so on stderr and keep stdout
		// clean instead of printing the renderer's "No results found" box.
		objectstorage.Hintf("no lifecycle rules on %s", lc.uri())
		return nil
	}
	render(lifecycleRuleRows(rules))
	return nil
}
