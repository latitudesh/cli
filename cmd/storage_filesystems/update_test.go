package storage_filesystems

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestBuildUpdateRequestSizeSet(t *testing.T) {
	req := buildUpdateRequest(true, 3000)
	if req.Data.Type != operations.PatchStorageFilesystemsFilesystemStorageTypeFilesystems {
		t.Errorf("Type = %q, want filesystems", req.Data.Type)
	}
	if req.Data.Attributes.SizeInGb == nil || *req.Data.Attributes.SizeInGb != 3000 {
		t.Errorf("SizeInGb = %v, want 3000", req.Data.Attributes.SizeInGb)
	}
}

func TestBuildUpdateRequestSizeOmitted(t *testing.T) {
	req := buildUpdateRequest(false, 0)
	if req.Data.Attributes.SizeInGb != nil {
		t.Errorf("SizeInGb = %v, want nil when --size not set", req.Data.Attributes.SizeInGb)
	}
}

func TestUpdateAndDeleteArgs(t *testing.T) {
	if err := NewUpdateCmd().Args(NewUpdateCmd(), []string{}); err == nil {
		t.Error("update: expected error with no args")
	}
	if err := NewUpdateCmd().Args(NewUpdateCmd(), []string{"fs_1"}); err != nil {
		t.Errorf("update: unexpected error with one arg: %v", err)
	}
	if err := NewDeleteCmd().Args(NewDeleteCmd(), []string{}); err == nil {
		t.Error("delete: expected error with no args")
	}
	if err := NewDeleteCmd().Args(NewDeleteCmd(), []string{"fs_1"}); err != nil {
		t.Errorf("delete: unexpected error with one arg: %v", err)
	}
}
