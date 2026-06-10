package cli

import (
	"context"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
)

func makeOperationGroupTeamMembersCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage members of the current team",
	}

	listCmd, err := makeOperationTeamMembersListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	addCmd, err := makeOperationTeamMembersAddCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(addCmd)

	removeCmd, err := makeOperationTeamMembersRemoveCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(removeCmd)

	return cmd, nil
}

func makeOperationTeamMembersListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List members of the current team",
		Example:      `  lsh teams members list`,
		Args:         cobra.NoArgs,
		RunE:         runTeamMembersList,
		SilenceUsage: true,
	}
	return cmd, nil
}

func makeOperationTeamMembersAddCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Invite a user to the current team",
		Example: `  lsh teams members add --email=user@example.com --role=collaborator
  lsh teams members add --email=user@example.com --role=administrator --first-name=Jane --last-name=Doe`,
		Args:         cobra.NoArgs,
		RunE:         runTeamMembersAdd,
		SilenceUsage: true,
	}

	cmd.Flags().String("email", "", "User email (required)")
	cmd.Flags().String("role", "", "Role: owner, administrator, collaborator or billing (required)")
	cmd.Flags().String("first-name", "", "User first name")
	cmd.Flags().String("last-name", "", "User last name")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("role")

	return cmd, nil
}

func makeOperationTeamMembersRemoveCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "remove <user-id>",
		Short:        "Remove a user from the current team",
		Example:      `  lsh teams members remove usr_xxx`,
		Args:         cobra.ExactArgs(1),
		RunE:         runTeamMembersRemove,
		SilenceUsage: true,
	}
	return cmd, nil
}

type teamMemberRow struct {
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Email      string `json:"email,omitempty"`
	Role       string `json:"role,omitempty"`
	MfaEnabled bool   `json:"mfa_enabled,omitempty"`
}

func (m teamMemberRow) TableRow() table.Row {
	mfa := "no"
	if m.MfaEnabled {
		mfa = "yes"
	}
	return table.Row{
		"email":      {Value: m.Email, Label: "Email"},
		"first_name": {Value: m.FirstName, Label: "First Name"},
		"last_name":  {Value: m.LastName, Label: "Last Name"},
		"role":       {Value: m.Role, Label: "Role"},
		"mfa":        {Value: mfa, Label: "MFA"},
	}
}

func runTeamMembersList(_ *cobra.Command, _ []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.Teams.Members.GetTeamMembers(ctx, nil, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip rendering.")
		return nil
	}

	if resp == nil || resp.TeamMembers == nil {
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.TeamMembers.Data))
	for i := range resp.TeamMembers.Data {
		rows = append(rows, teamMemberToRow(&resp.TeamMembers.Data[i]))
	}
	renderer.Render(rows)
	return nil
}

func runTeamMembersAdd(cmd *cobra.Command, _ []string) error {
	email, _ := cmd.Flags().GetString("email")
	roleStr, _ := cmd.Flags().GetString("role")
	firstName, _ := cmd.Flags().GetString("first-name")
	lastName, _ := cmd.Flags().GetString("last-name")

	role, err := parseTeamMemberRole(roleStr)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	attrs := &operations.PostTeamMembersTeamMembersAttributes{
		Email: email,
		Role:  role,
	}
	if firstName != "" {
		attrs.FirstName = &firstName
	}
	if lastName != "" {
		attrs.LastName = &lastName
	}

	body := operations.PostTeamMembersTeamMembersRequestBody{
		Data: operations.PostTeamMembersTeamMembersData{
			Type:       operations.PostTeamMembersTeamMembersTypeMemberships,
			Attributes: attrs,
		},
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.TeamMembers.PostTeamMembers(ctx, body)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.Membership == nil || resp.Membership.Data == nil {
		renderer.Render(nil)
		return nil
	}
	renderer.Render([]renderer.ResponseData{membershipToRow(resp.Membership.Data)})
	return nil
}

func runTeamMembersRemove(_ *cobra.Command, args []string) error {
	userID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	_, err := client.TeamMembers.Delete(ctx, userID)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	return nil
}

func parseTeamMemberRole(s string) (operations.PostTeamMembersRole, error) {
	switch strings.ToLower(s) {
	case "owner":
		return operations.PostTeamMembersRoleOwner, nil
	case "administrator":
		return operations.PostTeamMembersRoleAdministrator, nil
	case "collaborator":
		return operations.PostTeamMembersRoleCollaborator, nil
	case "billing":
		return operations.PostTeamMembersRoleBilling, nil
	default:
		return "", &invalidEnumError{
			field:   "role",
			value:   s,
			allowed: "owner, administrator, collaborator, billing",
		}
	}
}

func teamMemberToRow(m *components.TeamMembersData) teamMemberRow {
	row := teamMemberRow{}
	if m == nil || m.Attributes == nil {
		return row
	}
	attrs := m.Attributes
	if attrs.FirstName != nil {
		row.FirstName = *attrs.FirstName
	}
	if attrs.LastName != nil {
		row.LastName = *attrs.LastName
	}
	if attrs.Email != nil {
		row.Email = *attrs.Email
	}
	if attrs.MfaEnabled != nil {
		row.MfaEnabled = *attrs.MfaEnabled
	}
	if attrs.Role != nil && attrs.Role.Name != nil {
		row.Role = *attrs.Role.Name
	}
	return row
}

func membershipToRow(m *components.MembershipData) teamMemberRow {
	row := teamMemberRow{}
	if m == nil {
		return row
	}
	if m.Attributes == nil {
		return row
	}
	if m.Attributes.FirstName != nil {
		row.FirstName = *m.Attributes.FirstName
	}
	if m.Attributes.LastName != nil {
		row.LastName = *m.Attributes.LastName
	}
	if m.Attributes.Email != nil {
		row.Email = *m.Attributes.Email
	}
	if m.Attributes.MfaEnabled != nil {
		row.MfaEnabled = *m.Attributes.MfaEnabled
	}
	if m.Attributes.Role != nil {
		row.Role = string(*m.Attributes.Role)
	}
	return row
}
