package s3

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/spf13/cobra"
)

// Flag names shared by lifecycle create/update.
const (
	flagRuleName       = "name"
	flagRulePrefix     = "prefix"
	flagExpirationDays = "expiration-days"
	flagExpireDays     = "expire-days" // hidden alias of --expiration-days
	flagNoncurrentDays = "noncurrent-days"
	flagAbortMpuDays   = "abort-incomplete-multipart-days"
)

// lifecycleIDPrefix is the prefix of every lifecycle rule ID returned by the API.
const lifecycleIDPrefix = "lifecycle_"

// NewLifecycleCmd builds `lsh s3 lifecycle`, the bucket lifecycle rule group
// (list, get, create, update, delete). Rules are managed through the Latitude
// API; the bucket may be given as s3://<bucket> or <bucket>.
func NewLifecycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lifecycle",
		Aliases: []string{"lifecycle-rules"},
		Short:   "Manage bucket lifecycle rules",
		Long: `Manage the lifecycle rules of a bucket (automatic expiration of objects,
noncurrent versions and incomplete multipart uploads).

Rules are stored by the Latitude API, so no S3 credential is needed. Every
rule needs --expiration-days; rules can be addressed by id (lifecycle_...) or
by name in get/update/delete.`,
		Example: `  lsh s3 lifecycle create s3://logs --prefix tmp/ --expiration-days 7
  lsh s3 lifecycle create s3://logs --name abort-mpu --expiration-days 3650 --abort-incomplete-multipart-days 2
  lsh s3 lifecycle list s3://logs
  lsh s3 lifecycle update s3://logs expire-7d-tmp --expiration-days 14
  lsh s3 lifecycle delete s3://logs expire-7d-tmp`,
	}
	cmd.AddCommand(
		newLifecycleListCmd(),
		newLifecycleGetCmd(),
		newLifecycleCreateCmd(),
		newLifecycleUpdateCmd(),
		newLifecycleDeleteCmd(),
	)
	return cmd
}

// LifecycleRule wraps the SDK payload so the shared renderer can print it
// (-o table|json|yaml|csv, --query). JSON output is the API document as-is.
type LifecycleRule struct {
	components.LifecycleRuleData
}

// RuleID returns the rule id or "".
func (r LifecycleRule) RuleID() string { return ruleID(r.LifecycleRuleData) }

// RuleName returns the rule name or "".
func (r LifecycleRule) RuleName() string { return ruleName(r.LifecycleRuleData) }

// TableRow renders the rule as a table row: id, name, prefix, expiration,
// noncurrent, abort-mpu and enabled.
func (r LifecycleRule) TableRow() table.Row {
	a := r.Attributes
	return table.Row{
		"id":              {Label: "ID", Value: r.RuleID()},
		"name":            {Label: "Name", Value: r.RuleName()},
		"prefix":          {Label: "Prefix", Value: strOrDash(a.GetPrefix())},
		"expiration_days": {Label: "Expiration", Value: daysLabel(a.GetExpirationDays())},
		"noncurrent_days": {Label: "Noncurrent", Value: daysLabel(a.GetNoncurrentDays())},
		"abort_mpu_days":  {Label: "Abort MPU", Value: daysLabel(a.GetAbortMpuDaysAfterInitiation())},
		"enabled":         {Label: "Enabled", Value: boolLabel(a.GetEnabled())},
	}
}

// lifecycleRuleRows adapts SDK rules for the renderer.
func lifecycleRuleRows(rules []components.LifecycleRuleData) []renderer.ResponseData {
	out := make([]renderer.ResponseData, 0, len(rules))
	for _, r := range rules {
		out = append(out, LifecycleRule{LifecycleRuleData: r})
	}
	return out
}

// LifecycleMutation is the structured result of create/update/delete
// (create/update render the resulting rule; delete and dry-runs render this).
type LifecycleMutation struct {
	// Action is create, update or delete.
	Action string `json:"action"`
	Bucket string `json:"bucket"`
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (m LifecycleMutation) TableRow() table.Row {
	status := "ok"
	switch {
	case m.Error != "":
		status = "error: " + m.Error
	case m.DryRun:
		status = "dryrun"
	}
	return table.Row{
		"action": {Label: "Action", Value: m.Action},
		"bucket": {Label: "Bucket", Value: m.Bucket},
		"id":     {Label: "ID", Value: m.ID},
		"name":   {Label: "Name", Value: m.Name},
		"status": {Label: "Status", Value: status},
	}
}

// HumanLine renders the aws-style single line: `lifecycle: created <id> (<name>)`,
// or `(dryrun) lifecycle: create <name> on s3://bucket` for a dry-run.
func (m LifecycleMutation) HumanLine() string {
	subject := m.Name
	if m.ID != "" && m.Name != "" {
		subject = fmt.Sprintf("%s (%s)", m.ID, m.Name)
	} else if m.ID != "" {
		subject = m.ID
	}
	if m.DryRun {
		return fmt.Sprintf("(dryrun) lifecycle: %s %s on %s", m.Action, subject, m.Bucket)
	}
	if m.Error != "" {
		return fmt.Sprintf("lifecycle: failed to %s %s: %s", m.Action, subject, m.Error)
	}
	return fmt.Sprintf("lifecycle: %s %s", pastTense(m.Action), subject)
}

func pastTense(action string) string {
	switch action {
	case "create":
		return "created"
	case "update":
		return "updated"
	case "delete":
		return "deleted"
	}
	return action
}

// ruleID / ruleName read the optional SDK fields.
func ruleID(r components.LifecycleRuleData) string {
	if r.ID == nil {
		return ""
	}
	return *r.ID
}

func ruleName(r components.LifecycleRuleData) string {
	if r.Attributes == nil || r.Attributes.Name == nil {
		return ""
	}
	return *r.Attributes.Name
}

// daysLabel renders "7d", or the shared table placeholder when unset.
func daysLabel(v *int64) string {
	if v == nil {
		return emptyCell
	}
	return fmt.Sprintf("%dd", *v)
}

// strOrDash renders the string, or the shared table placeholder when unset.
func strOrDash(v *string) string {
	if v == nil {
		return emptyCell
	}
	return orEmptyCell(*v)
}

// boolLabel renders true/false, or the shared table placeholder when unset.
func boolLabel(v *bool) string {
	if v == nil {
		return emptyCell
	}
	if *v {
		return "true"
	}
	return "false"
}

// findLifecycleRule picks the rule addressed by token: an exact id match wins,
// otherwise the rule whose name equals token. It fails with exit 3 when
// nothing matches and exit 2 when several rules share the name.
func findLifecycleRule(rules []components.LifecycleRuleData, token string) (*components.LifecycleRuleData, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "missing lifecycle rule: pass the rule id (lifecycle_...) or its name")
	}
	for i := range rules {
		if ruleID(rules[i]) == token {
			return &rules[i], nil
		}
	}
	var matches []int
	for i := range rules {
		if ruleName(rules[i]) == token {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return nil, exitcode.Errorf(exitcode.NotFound, "lifecycle rule %q not found on this bucket; run 'lsh s3 lifecycle list' to see the rules", token)
	case 1:
		return &rules[matches[0]], nil
	}
	ids := make([]string, 0, len(matches))
	for _, i := range matches {
		ids = append(ids, "  "+ruleID(rules[i]))
	}
	sort.Strings(ids)
	return nil, exitcode.Errorf(exitcode.Usage, "lifecycle rule name %q matches several rules:\n%s\nuse the rule id instead", token, strings.Join(ids, "\n"))
}

// defaultLifecycleRuleName builds the default name of a new rule:
// `expire-<N>d`, plus `-<prefix>` (sanitized to [a-z0-9-]) when a prefix is set.
func defaultLifecycleRuleName(expirationDays int64, prefix string) string {
	name := fmt.Sprintf("expire-%dd", expirationDays)
	if s := sanitizeRuleNamePart(prefix); s != "" {
		name += "-" + s
	}
	return name
}

// sanitizeRuleNamePart lowercases s and collapses every run of characters
// outside [a-z0-9] into a single "-", trimming dashes at both ends.
func sanitizeRuleNamePart(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
			}
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// addLifecycleRuleFlags registers the attribute flags shared by create/update.
func addLifecycleRuleFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String(flagRuleName, "", "rule name (default: expire-<N>d[-<prefix>])")
	f.String(flagRulePrefix, "", "only apply to objects whose key starts with this prefix (default: whole bucket)")
	f.Int64(flagExpirationDays, 0, "delete objects this many days after creation (required by the API)")
	f.Int64(flagExpireDays, 0, "alias of --expiration-days")
	f.Int64(flagNoncurrentDays, 0, "delete noncurrent versions this many days after they become noncurrent (versioned buckets)")
	f.Int64(flagAbortMpuDays, 0, "abort incomplete multipart uploads this many days after initiation")
	_ = f.MarkHidden(flagExpireDays)
}

// expirationDaysFlag returns --expiration-days (or its alias --expire-days)
// and whether either was given.
func expirationDaysFlag(cmd *cobra.Command) (int64, bool) {
	if cmd.Flags().Changed(flagExpirationDays) {
		v, _ := cmd.Flags().GetInt64(flagExpirationDays)
		return v, true
	}
	if cmd.Flags().Changed(flagExpireDays) {
		v, _ := cmd.Flags().GetInt64(flagExpireDays)
		return v, true
	}
	return 0, false
}

// lifecycleAPI bundles the API client and the resolved bucket for one command.
type lifecycleAPI struct {
	api    *sdk.Latitudesh
	bucket *objectstorage.Bucket
	opts   []operations.Option
}

// openLifecycleAPI parses the bucket argument, resolves it through the API
// and returns the client used by every lifecycle subcommand.
func openLifecycleAPI(ctx context.Context, cmd *cobra.Command, arg string) (*lifecycleAPI, error) {
	ref, err := objectstorage.ParseBucketOnly(arg)
	if err != nil {
		return nil, err
	}
	if endpointOverride(cmd) != "" {
		return nil, exitcode.Errorf(exitcode.Usage, "lifecycle rules are managed through the Latitude API; unset --endpoint-url / %s to use this command", objectstorage.EnvEndpointURL)
	}
	b, err := resolveBucket(ctx, cmd, ref.Bucket)
	if err != nil {
		return nil, err
	}
	if b.EndpointOverride || b.ID == "" {
		return nil, exitcode.Errorf(exitcode.Usage, "bucket %s has no API id; lifecycle rules need the Latitude API", b.Display())
	}
	return &lifecycleAPI{
		api:    apiClient(),
		bucket: b,
		opts:   []operations.Option{operations.WithRetries(lsh.RetryConfig())},
	}, nil
}

// uri returns the s3:// form of the bucket for messages.
func (l *lifecycleAPI) uri() string { return "s3://" + l.bucket.Name }

// list returns every rule of the bucket (empty slice when there is none).
func (l *lifecycleAPI) list(ctx context.Context) ([]components.LifecycleRuleData, error) {
	resp, err := l.api.ObjectStorage.GetStorageBucketLifecycleRules(ctx, l.bucket.ID, l.opts...)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, fmt.Sprintf("bucket %s", l.bucket.Display()))
	}
	if resp.LifecycleRules == nil {
		return []components.LifecycleRuleData{}, nil
	}
	return resp.LifecycleRules.Data, nil
}

// get resolves token (lifecycle_ id or name) to a rule. Ids are fetched
// directly; names are looked up in the rule list.
func (l *lifecycleAPI) get(ctx context.Context, token string) (*components.LifecycleRuleData, error) {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(token, lifecycleIDPrefix) {
		resp, err := l.api.ObjectStorage.GetStorageBucketLifecycleRule(ctx, l.bucket.ID, token, l.opts...)
		switch {
		case err != nil && exitcode.Of(objectstorage.HumanizeAPI(err, "")) != exitcode.NotFound:
			return nil, objectstorage.HumanizeAPI(err, fmt.Sprintf("lifecycle rule %q on bucket %s", token, l.bucket.Display()))
		case err == nil && resp.Object != nil && resp.Object.Data != nil:
			return resp.Object.Data, nil
		}
		// Rule names are free text, so a name may start with the ID prefix:
		// fall through to the name lookup instead of reporting "not found".
	}
	rules, err := l.list(ctx)
	if err != nil {
		return nil, err
	}
	return findLifecycleRule(rules, token)
}

func (l *lifecycleAPI) create(ctx context.Context, body operations.PostStorageBucketLifecycleRulesRequestBody) (*components.LifecycleRuleData, error) {
	resp, err := l.api.ObjectStorage.PostStorageBucketLifecycleRules(ctx, l.bucket.ID, body, l.opts...)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, fmt.Sprintf("bucket %s", l.bucket.Display()))
	}
	if resp.Object == nil || resp.Object.Data == nil {
		return nil, exitcode.Errorf(exitcode.Generic, "the API returned no lifecycle rule; run 'lsh s3 lifecycle list %s' to check", l.uri())
	}
	return resp.Object.Data, nil
}

func (l *lifecycleAPI) update(ctx context.Context, id string, body operations.PatchStorageBucketLifecycleRuleRequestBody) (*components.LifecycleRuleData, error) {
	resp, err := l.api.ObjectStorage.PatchStorageBucketLifecycleRule(ctx, l.bucket.ID, id, body, l.opts...)
	if err != nil {
		return nil, objectstorage.HumanizeAPI(err, fmt.Sprintf("lifecycle rule %q on bucket %s", id, l.bucket.Display()))
	}
	if resp.Object == nil || resp.Object.Data == nil {
		return nil, exitcode.Errorf(exitcode.Generic, "the API returned no lifecycle rule; run 'lsh s3 lifecycle get %s %s' to check", l.uri(), id)
	}
	return resp.Object.Data, nil
}

func (l *lifecycleAPI) delete(ctx context.Context, id string) error {
	_, err := l.api.ObjectStorage.DeleteStorageBucketLifecycleRule(ctx, l.bucket.ID, id, l.opts...)
	if err != nil {
		return objectstorage.HumanizeAPI(err, fmt.Sprintf("lifecycle rule %q on bucket %s", id, l.bucket.Display()))
	}
	return nil
}
