package s3

import (
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func ptrS(s string) *string { return &s }
func ptrI(i int64) *int64   { return &i }
func ptrB(b bool) *bool     { return &b }

func rule(id, name string, exp *int64) components.LifecycleRuleData {
	return components.LifecycleRuleData{
		ID: ptrS(id),
		Attributes: &components.LifecycleRuleDataAttributes{
			Name:           ptrS(name),
			ExpirationDays: exp,
			Enabled:        ptrB(true),
		},
	}
}

func fixtureRules() []components.LifecycleRuleData {
	return []components.LifecycleRuleData{
		rule("lifecycle_a", "expire-7d-tmp", ptrI(7)),
		rule("lifecycle_b", "abort-mpu", ptrI(3650)),
		rule("lifecycle_c", "dup", ptrI(1)),
		rule("lifecycle_d", "dup", ptrI(2)),
		rule("lifecycle_e", "lifecycle_a", ptrI(9)), // a name that looks like an id
	}
}

func TestFindLifecycleRule(t *testing.T) {
	rules := fixtureRules()

	cases := []struct {
		token    string
		wantID   string
		wantCode int
	}{
		{"lifecycle_a", "lifecycle_a", 0}, // id wins over the rule named lifecycle_a
		{"lifecycle_b", "lifecycle_b", 0},
		{"expire-7d-tmp", "lifecycle_a", 0},
		{" abort-mpu ", "lifecycle_b", 0},
		{"dup", "", exitcode.Usage},
		{"missing", "", exitcode.NotFound},
		{"", "", exitcode.Usage},
	}
	for _, c := range cases {
		got, err := findLifecycleRule(rules, c.token)
		if c.wantCode != 0 {
			if err == nil {
				t.Errorf("findLifecycleRule(%q): expected error", c.token)
				continue
			}
			if exitcode.Of(err) != c.wantCode {
				t.Errorf("findLifecycleRule(%q): exit %d, want %d (%v)", c.token, exitcode.Of(err), c.wantCode, err)
			}
			if c.token == "dup" && (!strings.Contains(err.Error(), "lifecycle_c") || !strings.Contains(err.Error(), "lifecycle_d")) {
				t.Errorf("ambiguous error should list the candidate ids, got %q", err.Error())
			}
			continue
		}
		if err != nil {
			t.Errorf("findLifecycleRule(%q): unexpected error %v", c.token, err)
			continue
		}
		if ruleID(*got) != c.wantID {
			t.Errorf("findLifecycleRule(%q) = %s, want %s", c.token, ruleID(*got), c.wantID)
		}
	}

	if _, err := findLifecycleRule(nil, "anything"); exitcode.Of(err) != exitcode.NotFound {
		t.Errorf("empty rule list: exit %d, want %d", exitcode.Of(err), exitcode.NotFound)
	}
}

func TestDefaultLifecycleRuleName(t *testing.T) {
	cases := []struct {
		days   int64
		prefix string
		want   string
	}{
		{7, "", "expire-7d"},
		{7, "tmp/", "expire-7d-tmp"},
		{30, "Logs/2026/", "expire-30d-logs-2026"},
		{1, "a_b  c", "expire-1d-a-b-c"},
		{365, "///", "expire-365d"},
		{14, "Ünïcode/ok", "expire-14d-n-code-ok"},
	}
	for _, c := range cases {
		if got := defaultLifecycleRuleName(c.days, c.prefix); got != c.want {
			t.Errorf("defaultLifecycleRuleName(%d, %q) = %q, want %q", c.days, c.prefix, got, c.want)
		}
	}
}

func TestLifecycleRuleTableRow(t *testing.T) {
	r := LifecycleRule{LifecycleRuleData: components.LifecycleRuleData{
		ID: ptrS("lifecycle_9x4kQ"),
		Attributes: &components.LifecycleRuleDataAttributes{
			Name:           ptrS("expire-7d-tmp"),
			Prefix:         ptrS("tmp/"),
			ExpirationDays: ptrI(7),
			Enabled:        ptrB(true),
		},
	}}
	row := r.TableRow()
	want := map[string]string{
		"id":              "lifecycle_9x4kQ",
		"name":            "expire-7d-tmp",
		"prefix":          "tmp/",
		"expiration_days": "7d",
		"noncurrent_days": "-",
		"abort_mpu_days":  "-",
		"enabled":         "true",
	}
	for k, v := range want {
		if row[k].Value != v {
			t.Errorf("row[%q] = %q, want %q", k, row[k].Value, v)
		}
	}

	// A rule with no attributes must not panic and renders the shared
	// placeholder ("-", the same one every s3 table uses for empty cells).
	empty := LifecycleRule{LifecycleRuleData: components.LifecycleRuleData{ID: ptrS("lifecycle_x")}}
	for _, k := range []string{"enabled", "prefix", "expiration_days"} {
		if got := empty.TableRow()[k].Value; got != emptyCell {
			t.Errorf("empty rule %s = %q, want %q", k, got, emptyCell)
		}
	}
	// An empty prefix string is also a placeholder, not a blank cell.
	blank := LifecycleRule{LifecycleRuleData: components.LifecycleRuleData{Attributes: &components.LifecycleRuleDataAttributes{Prefix: ptrS("")}}}
	if got := blank.TableRow()["prefix"].Value; got != emptyCell {
		t.Errorf("blank prefix = %q, want %q", got, emptyCell)
	}
}

func TestLifecycleMutationHumanLine(t *testing.T) {
	cases := []struct {
		m    LifecycleMutation
		want string
	}{
		{LifecycleMutation{Action: "create", Bucket: "s3://logs", ID: "lifecycle_1", Name: "expire-7d"}, "lifecycle: created lifecycle_1 (expire-7d)"},
		{LifecycleMutation{Action: "update", Bucket: "s3://logs", ID: "lifecycle_1", Name: "expire-7d"}, "lifecycle: updated lifecycle_1 (expire-7d)"},
		{LifecycleMutation{Action: "delete", Bucket: "s3://logs", ID: "lifecycle_1", Name: "expire-7d"}, "lifecycle: deleted lifecycle_1 (expire-7d)"},
		{LifecycleMutation{Action: "create", Bucket: "s3://logs", Name: "expire-7d", DryRun: true}, "(dryrun) lifecycle: create expire-7d on s3://logs"},
		{LifecycleMutation{Action: "delete", Bucket: "s3://logs", ID: "lifecycle_1", Name: "n", DryRun: true}, "(dryrun) lifecycle: delete lifecycle_1 (n) on s3://logs"},
		{LifecycleMutation{Action: "delete", Bucket: "s3://logs", ID: "lifecycle_1", Name: "n", Error: "boom"}, "lifecycle: failed to delete lifecycle_1 (n): boom"},
	}
	for _, c := range cases {
		if got := c.m.HumanLine(); got != c.want {
			t.Errorf("HumanLine() = %q, want %q", got, c.want)
		}
	}
}
