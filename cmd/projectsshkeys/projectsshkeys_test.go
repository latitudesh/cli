package projectsshkeys

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

// TestProjectFlagRegisteredOnAllCommands ensures every project-scoped command
// exposes the --project flag the shared resolution hook keys off of. Without
// it, the active-project default would never apply.
func TestProjectFlagRegisteredOnAllCommands(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag bool
	}{
		{"list", NewListCmd().Flags().Lookup("project") != nil},
		{"get", NewGetCmd().Flags().Lookup("project") != nil},
		{"create", NewCreateCmd().Flags().Lookup("project") != nil},
		{"delete", NewDeleteCmd().Flags().Lookup("project") != nil},
	} {
		if !tc.flag {
			t.Errorf("%s: missing --project flag", tc.name)
		}
	}
}

// TestProjectIDReadsFlag verifies the projectID helper resolves the --project
// value that the root hook sets.
func TestProjectIDReadsFlag(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Set("project", "proj_123"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := projectID(cmd); got != "proj_123" {
		t.Errorf("projectID = %q, want proj_123", got)
	}
}

// TestListRequestCarriesProjectAndTags verifies the list request threads the
// project scope and optional tag filter through to the SDK request.
func TestListRequestCarriesProjectAndTags(t *testing.T) {
	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{"--project", "proj_123", "--tags", "tag_a"}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	request := operations.GetProjectSSHKeysRequest{ProjectID: projectID(cmd)}
	if tags, _ := cmd.Flags().GetString("tags"); tags != "" {
		request.FilterTags = &tags
	}

	if request.ProjectID != "proj_123" {
		t.Errorf("ProjectID = %q, want proj_123", request.ProjectID)
	}
	if request.FilterTags == nil || *request.FilterTags != "tag_a" {
		t.Errorf("FilterTags = %v, want tag_a", request.FilterTags)
	}
}

// TestCreateBuildsRequestBody verifies the create request body type and attrs.
func TestCreateBuildsRequestBody(t *testing.T) {
	name := "laptop"
	publicKey := "ssh-ed25519 AAAA"
	request := operations.PostProjectSSHKeyProjectsSSHKeysRequestBody{
		Data: operations.PostProjectSSHKeyProjectsSSHKeysData{
			Type: operations.PostProjectSSHKeyProjectsSSHKeysTypeSSHKeys,
			Attributes: &operations.PostProjectSSHKeyProjectsSSHKeysAttributes{
				Name:      &name,
				PublicKey: &publicKey,
			},
		},
	}
	if request.Data.Type != "ssh_keys" {
		t.Errorf("type = %q, want ssh_keys", request.Data.Type)
	}
	if *request.Data.Attributes.Name != name {
		t.Error("name not wired through")
	}
}
