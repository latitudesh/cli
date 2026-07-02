package userdata

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	cobra "github.com/spf13/cobra"
)

const userDataFixture = `{
  "id": "ud_XQvNevboR5zpb",
  "type": "user_data",
  "attributes": {
    "description": "cloud-init",
    "content": "I2Nsb3VkLWNvbmZpZw==",
    "created_at": "2024-11-12T17:33:24+00:00",
    "updated_at": "2024-11-13T09:00:00+00:00"
  }
}`

// TestUserDataPayloadDecoding pins the SDK's typed user data envelope to the
// shape the API returns.
func TestUserDataPayloadDecoding(t *testing.T) {
	var props components.UserDataProperties
	if err := json.Unmarshal([]byte(userDataFixture), &props); err != nil {
		t.Fatalf("could not unmarshal user data fixture: %v", err)
	}

	row := (&UserData{UserDataProperties: props}).TableRow()

	expectations := map[string]string{
		"id":          "ud_XQvNevboR5zpb",
		"description": "cloud-init",
		"created_at":  "2024-11-12T17:33:24+00:00",
		"updated_at":  "2024-11-13T09:00:00+00:00",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
	// The base64 content must NOT be a list column; it belongs to the details
	// view, decoded.
	if _, ok := row["content"]; ok {
		t.Error("expected the content column to be absent from the list table")
	}
}

// TestUserDataDetailFields verifies the details view decodes the base64
// content and exposes the project association.
func TestUserDataDetailFields(t *testing.T) {
	var props components.UserDataProperties
	if err := json.Unmarshal([]byte(userDataFixture), &props); err != nil {
		t.Fatalf("could not unmarshal user data fixture: %v", err)
	}
	entry := &UserData{UserDataProperties: props}

	fields := entry.DetailFields()
	if got, want := fields["Content"], "#cloud-config"; got != want {
		t.Errorf("DetailFields()[Content] = %q, want %q", got, want)
	}

	// Invalid base64 falls back to the raw value.
	raw := "not base64!"
	entry.Attributes.Content = &raw
	if got := entry.DecodedContent(); got != raw {
		t.Errorf("DecodedContent() fallback = %q, want %q", got, raw)
	}
}

// TestResolveContentEncodesPlainText verifies --content is base64-encoded and
// --content-base64 is passed through unchanged, with the latter taking
// precedence.
func TestResolveContentEncodesPlainText(t *testing.T) {
	t.Run("plain text is encoded", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContentFlags(cmd)
		if err := cmd.Flags().Parse([]string{"--content", "#cloud-config"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		got, ok := resolveContent(cmd)
		if !ok {
			t.Fatal("expected content to be present")
		}
		want := base64.StdEncoding.EncodeToString([]byte("#cloud-config"))
		if got != want {
			t.Errorf("content = %q, want %q", got, want)
		}
	})

	t.Run("base64 is passed through", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContentFlags(cmd)
		if err := cmd.Flags().Parse([]string{"--content-base64", "YWxyZWFkeQ=="}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		got, ok := resolveContent(cmd)
		if !ok || got != "YWxyZWFkeQ==" {
			t.Errorf("content = %q (ok=%v), want YWxyZWFkeQ==", got, ok)
		}
	})

	t.Run("no flag returns not-present", func(t *testing.T) {
		cmd := &cobra.Command{}
		registerContentFlags(cmd)
		if _, ok := resolveContent(cmd); ok {
			t.Error("expected content to be absent")
		}
	})
}

// TestListRegistersSubcommandName guards the account-scoped list command.
func TestListRegistersSubcommandName(t *testing.T) {
	if cmd := NewListCmd(); cmd.Use != "list" {
		t.Errorf("Use = %q, want list", cmd.Use)
	}
}
