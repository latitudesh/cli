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

const (
	flagRuleEnable  = "enable"
	flagRuleDisable = "disable"
)

// lifecycleUpdateInput holds only the attributes the user asked to change
// (nil = untouched), so the PATCH carries just those.
type lifecycleUpdateInput struct {
	Name           *string
	Prefix         *string
	ExpirationDays *int64
	NoncurrentDays *int64
	AbortMpuDays   *int64
	Enabled        *bool
}

func (in lifecycleUpdateInput) empty() bool {
	return in.Name == nil && in.Prefix == nil && in.ExpirationDays == nil &&
		in.NoncurrentDays == nil && in.AbortMpuDays == nil && in.Enabled == nil
}

// buildLifecycleUpdateRequest builds the PATCH body from the current rule and
// the changed attributes. The API requires Name on every PATCH, so the current
// name is sent when --name was not given.
func buildLifecycleUpdateRequest(current components.LifecycleRuleData, in lifecycleUpdateInput) (operations.PatchStorageBucketLifecycleRuleRequestBody, error) {
	var body operations.PatchStorageBucketLifecycleRuleRequestBody
	if in.empty() {
		return body, exitcode.Errorf(exitcode.Usage, "nothing to update: pass at least one of --%s, --%s, --%s, --%s, --%s, --%s or --%s",
			flagRuleName, flagRulePrefix, flagExpirationDays, flagNoncurrentDays, flagAbortMpuDays, flagRuleEnable, flagRuleDisable)
	}
	name := ruleName(current)
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if name == "" {
		return body, exitcode.Errorf(exitcode.Usage, "the rule needs a name: pass --%s", flagRuleName)
	}
	if in.ExpirationDays != nil && *in.ExpirationDays <= 0 {
		return body, exitcode.Errorf(exitcode.Usage, "the API requires --expiration-days on every rule (a positive number of days after which objects are deleted)")
	}
	if in.NoncurrentDays != nil && *in.NoncurrentDays < 0 {
		return body, exitcode.Errorf(exitcode.Usage, "--%s must be a positive number of days", flagNoncurrentDays)
	}
	if in.AbortMpuDays != nil && *in.AbortMpuDays < 0 {
		return body, exitcode.Errorf(exitcode.Usage, "--%s must be a positive number of days", flagAbortMpuDays)
	}
	body.Data = operations.PatchStorageBucketLifecycleRuleData{
		Type: operations.PatchStorageBucketLifecycleRuleTypeLifecycleRules,
		Attributes: operations.PatchStorageBucketLifecycleRuleAttributes{
			Name:                        name,
			Enabled:                     in.Enabled,
			Prefix:                      in.Prefix,
			ExpirationDays:              in.ExpirationDays,
			NoncurrentDays:              in.NoncurrentDays,
			AbortMpuDaysAfterInitiation: in.AbortMpuDays,
		},
	}
	return body, nil
}

// plannedLifecycleUpdate merges the PATCH into the current rule for --dry-run.
func plannedLifecycleUpdate(current components.LifecycleRuleData, body operations.PatchStorageBucketLifecycleRuleRequestBody) LifecycleRule {
	out := current
	attrs := components.LifecycleRuleDataAttributes{}
	if current.Attributes != nil {
		attrs = *current.Attributes
	}
	a := body.Data.Attributes
	name := a.Name
	attrs.Name = &name
	if a.Enabled != nil {
		attrs.Enabled = a.Enabled
	}
	if a.Prefix != nil {
		attrs.Prefix = a.Prefix
	}
	if a.ExpirationDays != nil {
		attrs.ExpirationDays = a.ExpirationDays
	}
	if a.NoncurrentDays != nil {
		attrs.NoncurrentDays = a.NoncurrentDays
	}
	if a.AbortMpuDaysAfterInitiation != nil {
		attrs.AbortMpuDaysAfterInitiation = a.AbortMpuDaysAfterInitiation
	}
	out.Attributes = &attrs
	return LifecycleRule{LifecycleRuleData: out}
}

// newLifecycleUpdateCmd builds `lsh s3 lifecycle update s3://bucket <rule-id|name>`.
func newLifecycleUpdateCmd() *cobra.Command {
	cmd := newCmd(&cobra.Command{
		Use:     "update <s3://bucket> <rule-id|name>",
		Aliases: []string{"edit", "set"},
		Short:   "Update a lifecycle rule",
		Long: `Update a lifecycle rule, addressed by id (lifecycle_...) or by name.

Only the flags you pass are changed; --enable/--disable toggle the rule.`,
		Example: `  lsh s3 lifecycle update s3://logs expire-7d-tmp --expiration-days 14
  lsh s3 lifecycle update s3://logs lifecycle_9x4kQ --disable
  lsh s3 lifecycle update s3://logs expire-7d-tmp --name expire-14d-tmp --prefix tmp/`,
		Args: cobra.ExactArgs(2),
		RunE: runLifecycleUpdate,
	})
	addProjectFlag(cmd, true, "project of the bucket (disambiguates names shared across projects)")
	addBucketFilterFlags(cmd)
	addLifecycleRuleFlags(cmd)
	cmd.Flags().Bool(flagRuleEnable, false, "enable the rule")
	cmd.Flags().Bool(flagRuleDisable, false, "disable the rule")
	return cmd
}

// lifecycleUpdateInputFromFlags collects only the flags that were set.
func lifecycleUpdateInputFromFlags(cmd *cobra.Command) lifecycleUpdateInput {
	in := lifecycleUpdateInput{}
	f := cmd.Flags()
	if f.Changed(flagRuleName) {
		v, _ := f.GetString(flagRuleName)
		in.Name = &v
	}
	if f.Changed(flagRulePrefix) {
		v, _ := f.GetString(flagRulePrefix)
		in.Prefix = &v
	}
	if v, ok := expirationDaysFlag(cmd); ok {
		in.ExpirationDays = &v
	}
	if f.Changed(flagNoncurrentDays) {
		v, _ := f.GetInt64(flagNoncurrentDays)
		in.NoncurrentDays = &v
	}
	if f.Changed(flagAbortMpuDays) {
		v, _ := f.GetInt64(flagAbortMpuDays)
		in.AbortMpuDays = &v
	}
	if v, _ := f.GetBool(flagRuleEnable); v {
		t := true
		in.Enabled = &t
	}
	if v, _ := f.GetBool(flagRuleDisable); v {
		fl := false
		in.Enabled = &fl
	}
	return in
}

func runLifecycleUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := objectstorage.SignalContext(context.Background())
	defer cancel()

	if en, _ := cmd.Flags().GetBool(flagRuleEnable); en {
		if dis, _ := cmd.Flags().GetBool(flagRuleDisable); dis {
			return printErr(exitcode.Errorf(exitcode.Usage, "--%s and --%s are mutually exclusive", flagRuleEnable, flagRuleDisable))
		}
	}
	in := lifecycleUpdateInputFromFlags(cmd)
	if in.empty() {
		// Fail before any API call; the builder produces the same message.
		_, err := buildLifecycleUpdateRequest(components.LifecycleRuleData{}, in)
		return printErr(err)
	}
	lc, err := openLifecycleAPI(ctx, cmd, args[0])
	if err != nil {
		return printErr(err)
	}
	current, err := lc.get(ctx, args[1])
	if err != nil {
		return printErr(err)
	}
	body, err := buildLifecycleUpdateRequest(*current, in)
	if err != nil {
		return printErr(err)
	}
	id := ruleID(*current)

	if dryRun() {
		if isHuman() {
			fmt.Println(LifecycleMutation{Action: "update", Bucket: lc.uri(), ID: id, Name: body.Data.Attributes.Name, DryRun: true}.HumanLine())
			return nil
		}
		render([]renderer.ResponseData{plannedLifecycleUpdate(*current, body)})
		return nil
	}

	updated, err := lc.update(ctx, id, body)
	if err != nil {
		return printErr(err)
	}
	if isHuman() {
		fmt.Println(LifecycleMutation{Action: "update", Bucket: lc.uri(), ID: ruleID(*updated), Name: ruleName(*updated)}.HumanLine())
		return nil
	}
	render([]renderer.ResponseData{LifecycleRule{LifecycleRuleData: *updated}})
	return nil
}
