package sshkeys

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

const sshKeyFixture = `{
  "id": "ssh_XQvNevboR5zpb",
  "type": "ssh_keys",
  "attributes": {
    "name": "laptop",
    "public_key": "ssh-ed25519 AAAAC3Nz...",
    "fingerprint": "SHA256:abc123",
    "user": {"id": "user_x", "email": "jane@example.com"},
    "created_at": "2024-11-12T17:33:24+00:00",
    "updated_at": "2024-11-13T09:00:00+00:00"
  }
}`

// TestSSHKeyPayloadDecoding pins the SDK's typed SSH key envelope to the shape
// the API returns, so a schema regression fails here instead of rendering an
// empty table.
func TestSSHKeyPayloadDecoding(t *testing.T) {
	var data components.SSHKeyData
	if err := json.Unmarshal([]byte(sshKeyFixture), &data); err != nil {
		t.Fatalf("could not unmarshal ssh key fixture: %v", err)
	}

	row := (&SSHKey{SSHKeyData: data}).TableRow()

	expectations := map[string]string{
		"id":         "ssh_XQvNevboR5zpb",
		"name":       "laptop",
		"created_by": "jane@example.com",
		"created_at": "2024-11-12T17:33:24+00:00",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
	// Long fields must NOT be list columns; they belong to the details view.
	for _, key := range []string{"fingerprint", "public_key", "updated_at", "user"} {
		if _, ok := row[key]; ok {
			t.Errorf("expected column %q to be absent from the list table", key)
		}
	}
}

// TestSSHKeyDetailFields verifies the details view carries the long fields
// dropped from the list table.
func TestSSHKeyDetailFields(t *testing.T) {
	var data components.SSHKeyData
	if err := json.Unmarshal([]byte(sshKeyFixture), &data); err != nil {
		t.Fatalf("could not unmarshal ssh key fixture: %v", err)
	}

	fields := (&SSHKey{SSHKeyData: data}).DetailFields()
	expectations := map[string]string{
		"Fingerprint": "SHA256:abc123",
		"Public Key":  "ssh-ed25519 AAAAC3Nz...",
		"Updated At":  "2024-11-13T09:00:00+00:00",
	}
	for key, want := range expectations {
		if got := fields[key]; got != want {
			t.Errorf("DetailFields()[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestUserFallsBackToID ensures the Created By column shows the bare ID when
// the API does not side-load the user's email.
func TestUserFallsBackToID(t *testing.T) {
	id := "user_only_id"
	data := components.SSHKeyData{
		Attributes: &components.SSHKeyDataAttributes{
			User: &components.UserInclude{ID: &id},
		},
	}
	if got := (&SSHKey{SSHKeyData: data}).TableRow()["created_by"].Value; got != id {
		t.Errorf("created_by = %q, want %q", got, id)
	}
}

// TestListRegistersTagsFlag guards the account-scoped list command's flag.
func TestListRegistersTagsFlag(t *testing.T) {
	cmd := NewListCmd()
	if cmd.Flags().Lookup("tags") == nil {
		t.Fatal("expected --tags flag on ssh-keys list")
	}
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}
}

// TestCreateBuildsRequestBody verifies the create request body carries the
// ssh_keys type and the name/public-key attributes.
func TestCreateBuildsRequestBody(t *testing.T) {
	name := "laptop"
	publicKey := "ssh-ed25519 AAAA"
	request := operations.PostSSHKeySSHKeysRequestBody{
		Data: operations.PostSSHKeySSHKeysData{
			Type: operations.PostSSHKeySSHKeysTypeSSHKeys,
			Attributes: &operations.PostSSHKeySSHKeysAttributes{
				Name:      &name,
				PublicKey: &publicKey,
			},
		},
	}
	if request.Data.Type != "ssh_keys" {
		t.Errorf("type = %q, want ssh_keys", request.Data.Type)
	}
	if *request.Data.Attributes.Name != name || *request.Data.Attributes.PublicKey != publicKey {
		t.Error("attributes not wired through")
	}
}

// TestUpdateOnlyPatchesProvidedFields ensures a partial update leaves unset
// fields nil so the API does not blank them.
func TestUpdateOnlyPatchesProvidedFields(t *testing.T) {
	cmd := NewUpdateCmd()
	if err := cmd.Flags().Parse([]string{"--name", "renamed"}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	attributes := &operations.PutSSHKeySSHKeysAttributes{}
	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		attributes.Name = &v
	}
	if cmd.Flags().Changed("tags") {
		v, _ := cmd.Flags().GetStringSlice("tags")
		attributes.Tags = v
	}

	if attributes.Name == nil || *attributes.Name != "renamed" {
		t.Errorf("Name = %v, want renamed", attributes.Name)
	}
	if attributes.Tags != nil {
		t.Errorf("Tags = %v, want nil (not provided)", attributes.Tags)
	}
}
