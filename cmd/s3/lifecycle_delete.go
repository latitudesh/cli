package s3

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/spf13/cobra"
)

const flagRuleAll = "all"

// newLifecycleDeleteCmd builds `lsh s3 lifecycle delete s3://bucket <rule-id|name>`.
func newLifecycleDeleteCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "delete <s3://bucket> [<rule-id|name>]",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a lifecycle rule (or all of them with --all)",
		Long: `Delete one lifecycle rule, addressed by id (lifecycle_...) or by name, or
every rule of the bucket with --all.

Deletion asks for confirmation; pass --yes in scripts (a non-interactive
session without --yes fails with exit 7 instead of hanging).`,
		Example: `  lsh s3 lifecycle delete s3://logs expire-7d-tmp
  lsh s3 lifecycle rm s3://logs lifecycle_9x4kQ --yes
  lsh s3 lifecycle delete s3://logs --all --yes`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runLifecycleDelete,
	})
	addProjectFlag(cmd, true, "project of the bucket (disambiguates names shared across projects)")
	addBucketFilterFlags(cmd)
	addYesFlag(cmd)
	cmd.Flags().Bool(flagRuleAll, false, "delete every lifecycle rule of the bucket")
	return cmd
}

func runLifecycleDelete(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	all, _ := cmd.Flags().GetBool(flagRuleAll)
	yes, _ := cmd.Flags().GetBool(flagYes)
	switch {
	case all && len(args) == 2:
		return printErr(exitcode.Errorf(exitcode.Usage, "pass either a rule (<rule-id|name>) or --all, not both"))
	case !all && len(args) == 1:
		return printErr(exitcode.Errorf(exitcode.Usage, "missing lifecycle rule: pass <rule-id|name>, or --all to delete every rule of the bucket"))
	}

	lc, err := openLifecycleAPI(ctx, cmd, args[0])
	if err != nil {
		return printErr(err)
	}

	var targets []components.LifecycleRuleData
	if all {
		targets, err = lc.list(ctx)
		if err != nil {
			return printErr(err)
		}
		if len(targets) == 0 {
			objectstorage.Hintf("no lifecycle rules on %s; nothing to delete", lc.uri())
			if !isHuman() {
				render(nil)
			}
			return nil
		}
	} else {
		rule, err := lc.get(ctx, args[1])
		if err != nil {
			return printErr(err)
		}
		targets = []components.LifecycleRuleData{*rule}
	}

	if dryRun() {
		rows := make([]renderer.ResponseData, 0, len(targets))
		for _, r := range targets {
			m := LifecycleMutation{Action: "delete", Bucket: lc.uri(), ID: ruleID(r), Name: ruleName(r), DryRun: true}
			if isHuman() {
				fmt.Println(m.HumanLine())
			}
			rows = append(rows, m)
		}
		if !isHuman() {
			render(rows)
		}
		return nil
	}

	question := fmt.Sprintf("Delete lifecycle rule %s (%s) from %s?", ruleName(targets[0]), ruleID(targets[0]), lc.uri())
	if all {
		question = fmt.Sprintf("Delete all %d lifecycle rules from %s?", len(targets), lc.uri())
	}
	if err := objectstorage.ConfirmOrRefuse(cmd, yes, question); err != nil {
		return printErr(err)
	}

	rows := make([]renderer.ResponseData, 0, len(targets))
	var failed int
	var firstErr error
	for _, r := range targets {
		m := LifecycleMutation{Action: "delete", Bucket: lc.uri(), ID: ruleID(r), Name: ruleName(r)}
		if err := lc.delete(ctx, m.ID); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			m.Error = err.Error()
			if isHuman() {
				objectstorage.Warnf("%s", m.HumanLine())
			}
		} else if isHuman() {
			fmt.Println(m.HumanLine())
		}
		rows = append(rows, m)
	}
	if !isHuman() {
		render(rows)
	}
	switch {
	case failed == 0:
		return nil
	case failed == len(targets) && len(targets) == 1:
		return printErr(firstErr)
	case failed == len(targets):
		return printErr(exitcode.Errorf(exitcode.Of(firstErr), "none of the %d lifecycle rules could be deleted: %v", failed, firstErr))
	}
	return printErr(exitcode.Errorf(exitcode.Partial, "%d of %d lifecycle rules could not be deleted", failed, len(targets)))
}
