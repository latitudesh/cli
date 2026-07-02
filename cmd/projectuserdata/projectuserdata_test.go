package projectuserdata

import (
	"encoding/base64"
	"testing"
)

// TestProjectFlagRegisteredOnAllCommands ensures every project-scoped user data
// command exposes the --project flag the shared resolution hook keys off of.
func TestProjectFlagRegisteredOnAllCommands(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag bool
	}{
		{"list", NewListCmd().Flags().Lookup("project") != nil},
		{"get", NewGetCmd().Flags().Lookup("project") != nil},
		{"create", NewCreateCmd().Flags().Lookup("project") != nil},
		{"update", NewUpdateCmd().Flags().Lookup("project") != nil},
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

// TestResolveContentEncodesPlainText mirrors the account-scoped behaviour:
// --content is base64-encoded, --content-base64 passes through.
func TestResolveContentEncodesPlainText(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{"--content", "#cloud-config"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, ok := resolveContent(cmd)
	if !ok {
		t.Fatal("expected content present")
	}
	if want := base64.StdEncoding.EncodeToString([]byte("#cloud-config")); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}
