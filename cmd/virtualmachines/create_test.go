package virtualmachines

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func TestBuildCreateRequest(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{
		"--plan", "vm-small",
		"--project", "my-project",
		"--name", "my-vm",
		"--os", "ubuntu-24-04",
		"--ssh-keys", "key_a",
		"--ssh-keys", "key_b",
		"--tags", "tag_a,tag_b",
		"--user-data", "ud_xxx",
	}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	req, err := buildCreateRequest(cmd)
	if err != nil {
		t.Fatalf("buildCreateRequest returned error: %v", err)
	}

	attrs := req.Data.Attributes
	if attrs == nil {
		t.Fatal("expected attributes to be set")
	}
	if attrs.Plan == nil || *attrs.Plan != "vm-small" {
		t.Errorf("Plan = %v, want vm-small", attrs.Plan)
	}
	if attrs.Project == nil || *attrs.Project != "my-project" {
		t.Errorf("Project = %v, want my-project", attrs.Project)
	}
	if attrs.Name == nil || *attrs.Name != "my-vm" {
		t.Errorf("Name = %v, want my-vm", attrs.Name)
	}
	if attrs.OperatingSystem == nil || *attrs.OperatingSystem != "ubuntu-24-04" {
		t.Errorf("OperatingSystem = %v, want ubuntu-24-04", attrs.OperatingSystem)
	}
	if len(attrs.SSHKeys) != 2 || attrs.SSHKeys[0] != "key_a" || attrs.SSHKeys[1] != "key_b" {
		t.Errorf("SSHKeys = %v, want [key_a key_b]", attrs.SSHKeys)
	}
	if len(attrs.Tags) != 2 || attrs.Tags[0] != "tag_a" || attrs.Tags[1] != "tag_b" {
		t.Errorf("Tags = %v, want [tag_a tag_b]", attrs.Tags)
	}
	if attrs.UserData == nil || attrs.UserData.Str == nil || *attrs.UserData.Str != "ud_xxx" {
		t.Errorf("UserData = %v, want ud_xxx", attrs.UserData)
	}
	if req.Data.Type == nil || *req.Data.Type != components.VirtualMachinePayloadTypeVirtualMachines {
		t.Errorf("Type = %v, want virtual_machines", req.Data.Type)
	}
}

func TestBuildCreateRequestRequiresPlanAndProject(t *testing.T) {
	cases := [][]string{
		{"--project", "my-project"}, // missing plan
		{"--plan", "vm-small"},      // missing project
		{},                          // missing both
	}
	for _, args := range cases {
		cmd := NewCreateCmd()
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatalf("flag parse error: %v", err)
		}
		if _, err := buildCreateRequest(cmd); err == nil {
			t.Errorf("expected error for args %v, got nil", args)
		}
	}
}

func TestBuildUpdateRequest(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--name", "renamed", "--tags", "tag_a,tag_b"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	req, err := buildUpdateRequest(cmd, "vm_x")
	if err != nil {
		t.Fatalf("buildUpdateRequest returned error: %v", err)
	}
	if req.Data.ID == nil || *req.Data.ID != "vm_x" {
		t.Errorf("ID = %v, want vm_x", req.Data.ID)
	}
	if req.Data.Attributes.Name == nil || *req.Data.Attributes.Name != "renamed" {
		t.Errorf("Name = %v, want renamed", req.Data.Attributes.Name)
	}
	if len(req.Data.Attributes.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 entries", req.Data.Attributes.Tags)
	}
	if req.Data.Type != components.VirtualMachineUpdatePayloadTypeVirtualMachines {
		t.Errorf("Type = %v, want virtual_machines", req.Data.Type)
	}
}

func TestBuildUpdateRequestNoFields(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if _, err := buildUpdateRequest(cmd, "vm_x"); err == nil {
		t.Error("expected error when no fields are supplied, got nil")
	}
}

func TestBuildActionRequest(t *testing.T) {
	cmd := NewActionCmd()
	if err := cmd.Flags().Parse([]string{"--action", "reboot"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	body, err := buildActionRequest(cmd)
	if err != nil {
		t.Fatalf("buildActionRequest returned error: %v", err)
	}
	if body.Attributes.Action != "reboot" {
		t.Errorf("Action = %v, want reboot", body.Attributes.Action)
	}
}

func TestBuildActionRequestInvalid(t *testing.T) {
	for _, action := range []string{"", "explode"} {
		cmd := NewActionCmd()
		args := []string{}
		if action != "" {
			args = []string{"--action", action}
		}
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatalf("flag parse error: %v", err)
		}
		if _, err := buildActionRequest(cmd); err == nil {
			t.Errorf("expected error for action %q, got nil", action)
		}
	}
}
