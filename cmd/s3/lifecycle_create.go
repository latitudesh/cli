package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/spf13/cobra"
)

const flagRuleDisabled = "disabled"

// lifecycleCreateInput is the flag set of `lifecycle create`, kept separate
// from cobra so the request builder is testable.
type lifecycleCreateInput struct {
	Name           string
	Prefix         string
	ExpirationDays int64
	NoncurrentDays int64
	AbortMpuDays   int64
	Disabled       bool
}

// buildLifecycleCreateRequest validates the input and builds the POST body.
// --expiration-days is mandatory on the API side, so it is checked here with
// a clear message instead of a 422.
func buildLifecycleCreateRequest(in lifecycleCreateInput) (operations.PostStorageBucketLifecycleRulesRequestBody, error) {
	var body operations.PostStorageBucketLifecycleRulesRequestBody
	if in.ExpirationDays <= 0 {
		return body, exitcode.Errorf(exitcode.Usage, "the API requires --expiration-days on every rule (a positive number of days after which objects are deleted)")
	}
	if in.NoncurrentDays < 0 {
		return body, exitcode.Errorf(exitcode.Usage, "--%s must be a positive number of days", flagNoncurrentDays)
	}
	if in.AbortMpuDays < 0 {
		return body, exitcode.Errorf(exitcode.Usage, "--%s must be a positive number of days", flagAbortMpuDays)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = defaultLifecycleRuleName(in.ExpirationDays, in.Prefix)
	}
	enabled := !in.Disabled
	attrs := operations.PostStorageBucketLifecycleRulesAttributes{
		Name:           name,
		Enabled:        &enabled,
		ExpirationDays: in.ExpirationDays,
	}
	if p := in.Prefix; p != "" {
		attrs.Prefix = &p
	}
	if in.NoncurrentDays > 0 {
		v := in.NoncurrentDays
		attrs.NoncurrentDays = &v
	}
	if in.AbortMpuDays > 0 {
		v := in.AbortMpuDays
		attrs.AbortMpuDaysAfterInitiation = &v
	}
	body.Data = operations.PostStorageBucketLifecycleRulesData{
		Type:       operations.PostStorageBucketLifecycleRulesTypeLifecycleRules,
		Attributes: attrs,
	}
	return body, nil
}

// plannedLifecycleRule turns a create request into a renderable rule for
// --dry-run (no id, since nothing was created).
func plannedLifecycleRule(body operations.PostStorageBucketLifecycleRulesRequestBody) LifecycleRule {
	a := body.Data.Attributes
	name := a.Name
	exp := a.ExpirationDays
	typ := components.LifecycleRuleDataTypeLifecycleRules
	return LifecycleRule{LifecycleRuleData: components.LifecycleRuleData{
		Type: &typ,
		Attributes: &components.LifecycleRuleDataAttributes{
			Name:                        &name,
			Enabled:                     a.Enabled,
			Prefix:                      a.Prefix,
			ExpirationDays:              &exp,
			NoncurrentDays:              a.NoncurrentDays,
			AbortMpuDaysAfterInitiation: a.AbortMpuDaysAfterInitiation,
		},
	}}
}

// newLifecycleCreateCmd builds `lsh s3 lifecycle create s3://bucket`.
func newLifecycleCreateCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "create <s3://bucket>",
		Aliases: []string{"add"},
		Short:   "Create a lifecycle rule",
		Long: `Create a lifecycle rule on a bucket.

--expiration-days is required by the API on every rule, even when the goal is
only to abort incomplete multipart uploads or expire noncurrent versions. The
default name is expire-<N>d, with -<prefix> appended when --prefix is given.`,
		Example: `  lsh s3 lifecycle create s3://logs --prefix tmp/ --expiration-days 7
  lsh s3 lifecycle create s3://logs --name abort-mpu --expiration-days 3650 --abort-incomplete-multipart-days 2
  lsh s3 lifecycle create s3://backups --expiration-days 90 --noncurrent-days 30 --disabled`,
		Args: cobra.ExactArgs(1),
		RunE: runLifecycleCreate,
	})
	addProjectFlag(cmd, true, "project of the bucket (disambiguates names shared across projects)")
	addBucketFilterFlags(cmd)
	addLifecycleRuleFlags(cmd)
	cmd.Flags().Bool(flagRuleDisabled, false, "create the rule disabled")
	return cmd
}

func runLifecycleCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	in := lifecycleCreateInput{}
	in.Name, _ = cmd.Flags().GetString(flagRuleName)
	in.Prefix, _ = cmd.Flags().GetString(flagRulePrefix)
	in.ExpirationDays, _ = expirationDaysFlag(cmd)
	in.NoncurrentDays, _ = cmd.Flags().GetInt64(flagNoncurrentDays)
	in.AbortMpuDays, _ = cmd.Flags().GetInt64(flagAbortMpuDays)
	in.Disabled, _ = cmd.Flags().GetBool(flagRuleDisabled)

	// Validate the flags before touching the API so usage errors are cheap.
	body, err := buildLifecycleCreateRequest(in)
	if err != nil {
		return printErr(err)
	}
	lc, err := openLifecycleAPI(ctx, cmd, args[0])
	if err != nil {
		return printErr(err)
	}

	if dryRun() {
		if isHuman() {
			fmt.Println(LifecycleMutation{Action: "create", Bucket: lc.uri(), Name: body.Data.Attributes.Name, DryRun: true}.HumanLine())
			return nil
		}
		render([]renderer.ResponseData{plannedLifecycleRule(body)})
		return nil
	}

	created, err := lc.create(ctx, body)
	if err != nil {
		return printErr(err)
	}
	if isHuman() {
		fmt.Println(LifecycleMutation{Action: "create", Bucket: lc.uri(), ID: ruleID(*created), Name: ruleName(*created)}.HumanLine())
		return nil
	}
	render([]renderer.ResponseData{LifecycleRule{LifecycleRuleData: *created}})
	return nil
}
