package s3

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func TestBuildLifecycleCreateRequest(t *testing.T) {
	body, err := buildLifecycleCreateRequest(lifecycleCreateInput{Prefix: "tmp/", ExpirationDays: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := body.Data.Attributes
	if body.Data.Type != operations.PostStorageBucketLifecycleRulesTypeLifecycleRules {
		t.Errorf("type = %q", body.Data.Type)
	}
	if a.Name != "expire-7d-tmp" {
		t.Errorf("default name = %q, want expire-7d-tmp", a.Name)
	}
	if a.ExpirationDays != 7 {
		t.Errorf("expiration_days = %d", a.ExpirationDays)
	}
	if a.Prefix == nil || *a.Prefix != "tmp/" {
		t.Errorf("prefix = %v", a.Prefix)
	}
	if a.Enabled == nil || !*a.Enabled {
		t.Errorf("enabled should default to true, got %v", a.Enabled)
	}
	if a.NoncurrentDays != nil || a.AbortMpuDaysAfterInitiation != nil {
		t.Errorf("optional days must be omitted when zero: %v %v", a.NoncurrentDays, a.AbortMpuDaysAfterInitiation)
	}

	// Explicit name, all optional attributes and --disabled.
	body, err = buildLifecycleCreateRequest(lifecycleCreateInput{Name: " abort-mpu ", ExpirationDays: 3650, NoncurrentDays: 30, AbortMpuDays: 2, Disabled: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a = body.Data.Attributes
	if a.Name != "abort-mpu" {
		t.Errorf("name = %q", a.Name)
	}
	if a.Prefix != nil {
		t.Errorf("prefix should be omitted, got %q", *a.Prefix)
	}
	if a.NoncurrentDays == nil || *a.NoncurrentDays != 30 {
		t.Errorf("noncurrent_days = %v", a.NoncurrentDays)
	}
	if a.AbortMpuDaysAfterInitiation == nil || *a.AbortMpuDaysAfterInitiation != 2 {
		t.Errorf("abort_mpu_days_after_initiation = %v", a.AbortMpuDaysAfterInitiation)
	}
	if a.Enabled == nil || *a.Enabled {
		t.Errorf("enabled should be false with --disabled, got %v", a.Enabled)
	}

	// The wire format uses the API attribute names.
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"lifecycle_rules"`, `"expiration_days":3650`, `"noncurrent_days":30`, `"abort_mpu_days_after_initiation":2`, `"enabled":false`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("payload %s lacks %s", raw, want)
		}
	}
}

func TestBuildLifecycleCreateRequestValidation(t *testing.T) {
	cases := []struct {
		name string
		in   lifecycleCreateInput
		want string
	}{
		{"missing expiration", lifecycleCreateInput{AbortMpuDays: 2}, "the API requires --expiration-days on every rule"},
		{"negative expiration", lifecycleCreateInput{ExpirationDays: -1}, "the API requires --expiration-days on every rule"},
		{"negative noncurrent", lifecycleCreateInput{ExpirationDays: 1, NoncurrentDays: -1}, "--noncurrent-days"},
		{"negative abort", lifecycleCreateInput{ExpirationDays: 1, AbortMpuDays: -5}, "--abort-incomplete-multipart-days"},
	}
	for _, c := range cases {
		_, err := buildLifecycleCreateRequest(c.in)
		if err == nil {
			t.Errorf("%s: expected error", c.name)
			continue
		}
		if exitcode.Of(err) != exitcode.Usage {
			t.Errorf("%s: exit %d, want %d", c.name, exitcode.Of(err), exitcode.Usage)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q should mention %q", c.name, err.Error(), c.want)
		}
	}
}

func TestPlannedLifecycleRule(t *testing.T) {
	body, err := buildLifecycleCreateRequest(lifecycleCreateInput{Prefix: "tmp/", ExpirationDays: 7, AbortMpuDays: 2})
	if err != nil {
		t.Fatal(err)
	}
	row := plannedLifecycleRule(body).TableRow()
	if row["id"].Value != "" {
		t.Errorf("dry-run rule must have no id, got %q", row["id"].Value)
	}
	if row["name"].Value != "expire-7d-tmp" || row["expiration_days"].Value != "7d" || row["abort_mpu_days"].Value != "2d" || row["noncurrent_days"].Value != emptyCell { // unset days render the shared "-" placeholder
		t.Errorf("unexpected planned row: %+v", row)
	}
}
