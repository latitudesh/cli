package storage_objects

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestBuildCreateRequest(t *testing.T) {
	req, err := buildCreateRequest("my-project", "my-bucket", "SAO2", "high_performance", true, true, false, false)
	if err != nil {
		t.Fatalf("buildCreateRequest returned error: %v", err)
	}

	if req.Data.Type != operations.PostStorageBucketsTypeObjects {
		t.Errorf("Type = %q, want objects", req.Data.Type)
	}
	attr := req.Data.Attributes
	if attr.Project != "my-project" || attr.Name != "my-bucket" || attr.Region != "SAO2" {
		t.Errorf("unexpected core attributes: %+v", attr)
	}
	if attr.StorageClass == nil || *attr.StorageClass != operations.StorageClassHighPerformance {
		t.Errorf("StorageClass = %v, want high_performance", attr.StorageClass)
	}
	if attr.Versioning == nil || !*attr.Versioning {
		t.Errorf("Versioning = %v, want true", attr.Versioning)
	}
	if attr.Locking != nil {
		t.Errorf("Locking = %v, want nil when --locking not set", attr.Locking)
	}
}

func TestBuildCreateRequestDefaults(t *testing.T) {
	req, err := buildCreateRequest("p", "b", "DAL", "", false, false, false, false)
	if err != nil {
		t.Fatalf("buildCreateRequest returned error: %v", err)
	}
	attr := req.Data.Attributes
	if attr.StorageClass != nil {
		t.Errorf("StorageClass = %v, want nil by default", attr.StorageClass)
	}
	if attr.Versioning != nil || attr.Locking != nil {
		t.Errorf("optional toggles should be nil by default, got versioning=%v locking=%v", attr.Versioning, attr.Locking)
	}
}

func TestBuildCreateRequestValidation(t *testing.T) {
	cases := []struct {
		name                        string
		project, bucket, region, sc string
	}{
		{"missing project", "", "b", "DAL", ""},
		{"missing name", "p", "", "DAL", ""},
		{"missing region", "p", "b", "", ""},
		{"invalid storage-class", "p", "b", "DAL", "turbo"},
	}
	for _, c := range cases {
		if _, err := buildCreateRequest(c.project, c.bucket, c.region, c.sc, false, false, false, false); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestCreateCmdFlags(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{
		"--project", "p1",
		"--name", "b1",
		"--region", "SAO2",
		"--storage-class", "standard",
		"--versioning",
	}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if !cmd.Flags().Changed("versioning") {
		t.Error("expected --versioning to be marked changed")
	}
	if cmd.Flags().Changed("locking") {
		t.Error("expected --locking to be unset")
	}
}
