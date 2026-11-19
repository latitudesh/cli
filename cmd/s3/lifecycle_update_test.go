package s3

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func TestBuildLifecycleUpdateRequestPartial(t *testing.T) {
	current := components.LifecycleRuleData{
		ID: ptrS("lifecycle_a"),
		Attributes: &components.LifecycleRuleDataAttributes{
			Name:           ptrS("expire-7d-tmp"),
			Prefix:         ptrS("tmp/"),
			ExpirationDays: ptrI(7),
			Enabled:        ptrB(true),
		},
	}

	// Only --expiration-days changed: the name is filled from the rule and
	// nothing else is sent.
	body, err := buildLifecycleUpdateRequest(current, lifecycleUpdateInput{ExpirationDays: ptrI(14)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := body.Data.Attributes
	if a.Name != "expire-7d-tmp" {
		t.Errorf("name = %q, want the current name", a.Name)
	}
	if a.ExpirationDays == nil || *a.ExpirationDays != 14 {
		t.Errorf("expiration_days = %v", a.ExpirationDays)
	}
	if a.Prefix != nil || a.Enabled != nil || a.NoncurrentDays != nil || a.AbortMpuDaysAfterInitiation != nil {
		t.Errorf("untouched attributes must stay nil: %+v", a)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), `"prefix"`) || strings.Contains(string(raw), `"enabled"`) {
		t.Errorf("payload should only carry name and expiration_days: %s", raw)
	}

	// --disable plus a new name and an empty prefix (clears the filter).
	body, err = buildLifecycleUpdateRequest(current, lifecycleUpdateInput{Name: ptrS(" new-name "), Prefix: ptrS(""), Enabled: ptrB(false)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a = body.Data.Attributes
	if a.Name != "new-name" {
		t.Errorf("name = %q", a.Name)
	}
	if a.Enabled == nil || *a.Enabled {
		t.Errorf("enabled = %v, want false", a.Enabled)
	}
	if a.Prefix == nil || *a.Prefix != "" {
		t.Errorf("prefix = %v, want empty string", a.Prefix)
	}
	raw, _ = json.Marshal(body)
	if !strings.Contains(string(raw), `"prefix":""`) || !strings.Contains(string(raw), `"enabled":false`) {
		t.Errorf("payload should carry prefix and enabled: %s", raw)
	}
}

func TestBuildLifecycleUpdateRequestValidation(t *testing.T) {
	current := components.LifecycleRuleData{ID: ptrS("lifecycle_a"), Attributes: &components.LifecycleRuleDataAttributes{Name: ptrS("n")}}

	if _, err := buildLifecycleUpdateRequest(current, lifecycleUpdateInput{}); err == nil || exitcode.Of(err) != exitcode.Usage || !strings.Contains(err.Error(), "nothing to update") {
		t.Errorf("empty input: got %v", err)
	}
	if _, err := buildLifecycleUpdateRequest(current, lifecycleUpdateInput{ExpirationDays: ptrI(0)}); err == nil || !strings.Contains(err.Error(), "the API requires --expiration-days on every rule") {
		t.Errorf("zero expiration: got %v", err)
	}
	if _, err := buildLifecycleUpdateRequest(current, lifecycleUpdateInput{NoncurrentDays: ptrI(-1)}); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Errorf("negative noncurrent: got %v", err)
	}
	// A rule without a name needs --name.
	nameless := components.LifecycleRuleData{ID: ptrS("lifecycle_b"), Attributes: &components.LifecycleRuleDataAttributes{}}
	if _, err := buildLifecycleUpdateRequest(nameless, lifecycleUpdateInput{Enabled: ptrB(true)}); err == nil || !strings.Contains(err.Error(), "--name") {
		t.Errorf("nameless rule: got %v", err)
	}
	if body, err := buildLifecycleUpdateRequest(nameless, lifecycleUpdateInput{Name: ptrS("fixed"), Enabled: ptrB(true)}); err != nil || body.Data.Attributes.Name != "fixed" {
		t.Errorf("nameless rule with --name: %v %+v", err, body)
	}
}

func TestPlannedLifecycleUpdate(t *testing.T) {
	current := components.LifecycleRuleData{
		ID: ptrS("lifecycle_a"),
		Attributes: &components.LifecycleRuleDataAttributes{
			Name:           ptrS("expire-7d-tmp"),
			Prefix:         ptrS("tmp/"),
			ExpirationDays: ptrI(7),
			Enabled:        ptrB(true),
		},
	}
	body, err := buildLifecycleUpdateRequest(current, lifecycleUpdateInput{ExpirationDays: ptrI(14), Enabled: ptrB(false)})
	if err != nil {
		t.Fatal(err)
	}
	row := plannedLifecycleUpdate(current, body).TableRow()
	if row["id"].Value != "lifecycle_a" || row["name"].Value != "expire-7d-tmp" || row["prefix"].Value != "tmp/" {
		t.Errorf("planned update lost current values: %+v", row)
	}
	if row["expiration_days"].Value != "14d" || row["enabled"].Value != "false" {
		t.Errorf("planned update did not apply the patch: %+v", row)
	}
	// The original must not be mutated.
	if *current.Attributes.ExpirationDays != 7 || !*current.Attributes.Enabled {
		t.Errorf("plannedLifecycleUpdate mutated the current rule")
	}
}
