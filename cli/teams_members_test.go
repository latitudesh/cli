package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestParseTeamMemberRole(t *testing.T) {
	cases := []struct {
		in      string
		want    operations.PostTeamMembersRole
		wantErr bool
	}{
		{in: "owner", want: operations.PostTeamMembersRoleOwner},
		{in: "administrator", want: operations.PostTeamMembersRoleAdministrator},
		{in: "Administrator", want: operations.PostTeamMembersRoleAdministrator},
		{in: "collaborator", want: operations.PostTeamMembersRoleCollaborator},
		{in: "billing", want: operations.PostTeamMembersRoleBilling},
		{in: "BILLING", want: operations.PostTeamMembersRoleBilling},
		{in: "admin", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseTeamMemberRole(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseTeamMemberRole(%q): expected error, got %q", tc.in, got)
			} else if !strings.Contains(err.Error(), "owner, administrator, collaborator, billing") {
				t.Errorf("parseTeamMemberRole(%q): error should list allowed values, got %q", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTeamMemberRole(%q): unexpected error: %v", tc.in, err)
		} else if got != tc.want {
			t.Errorf("parseTeamMemberRole(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTeamMemberToRow(t *testing.T) {
	const fixture = `{
		"id": "usr_123",
		"attributes": {
			"first_name": "Jane",
			"last_name": "Doe",
			"email": "jane@example.com",
			"mfa_enabled": true,
			"role": {"name": "administrator"}
		}
	}`
	member := &components.TeamMembersData{}
	if err := json.Unmarshal([]byte(fixture), member); err != nil {
		t.Fatal(err)
	}

	row := teamMemberToRow(member)
	want := teamMemberRow{FirstName: "Jane", LastName: "Doe", Email: "jane@example.com", Role: "administrator", MfaEnabled: true}
	if row != want {
		t.Errorf("teamMemberToRow = %+v, want %+v", row, want)
	}
}

func TestTeamMemberToRow_NilSafety(t *testing.T) {
	if row := teamMemberToRow(nil); row != (teamMemberRow{}) {
		t.Errorf("teamMemberToRow(nil) = %+v, want zero row", row)
	}
	if row := teamMemberToRow(&components.TeamMembersData{}); row != (teamMemberRow{}) {
		t.Errorf("teamMemberToRow without attributes = %+v, want zero row", row)
	}
}

func TestMembershipToRow(t *testing.T) {
	const fixture = `{
		"id": "mem_123",
		"attributes": {
			"first_name": "John",
			"last_name": "Smith",
			"email": "john@example.com",
			"role": "collaborator",
			"mfa_enabled": false
		}
	}`
	membership := &components.MembershipData{}
	if err := json.Unmarshal([]byte(fixture), membership); err != nil {
		t.Fatal(err)
	}

	row := membershipToRow(membership)
	want := teamMemberRow{FirstName: "John", LastName: "Smith", Email: "john@example.com", Role: "collaborator"}
	if row != want {
		t.Errorf("membershipToRow = %+v, want %+v", row, want)
	}

	if got := membershipToRow(nil); got != (teamMemberRow{}) {
		t.Errorf("membershipToRow(nil) = %+v, want zero row", got)
	}
}

func TestTeamMemberRowTableRow_MFA(t *testing.T) {
	enabled := teamMemberRow{MfaEnabled: true}.TableRow()
	if enabled["mfa"].Value != "yes" {
		t.Errorf("MFA enabled: TableRow mfa = %q, want %q", enabled["mfa"].Value, "yes")
	}
	disabled := teamMemberRow{}.TableRow()
	if disabled["mfa"].Value != "no" {
		t.Errorf("MFA disabled: TableRow mfa = %q, want %q", disabled["mfa"].Value, "no")
	}
}
