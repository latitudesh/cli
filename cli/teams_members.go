package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
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

	cmd.Flags().String("email", "", "User email (prompted interactively when omitted)")
	cmd.Flags().String("role", "", "Role: owner, administrator, collaborator or billing (prompted interactively when omitted)")
	cmd.Flags().String("first-name", "", "User first name")
	cmd.Flags().String("last-name", "", "User last name")

	return cmd, nil
}

func makeOperationTeamMembersRemoveCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "remove [user-id]",
		Short: "Remove a user from the current team",
		Example: `  lsh teams members remove usr_xxx
  lsh teams members remove   # pick the member interactively`,
		Args:         cobra.MaximumNArgs(1),
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
	MfaEnabled bool   `json:"mfa_enabled"`
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
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	stopSpinner := tui.StartFetchSpinner("Fetching team members…")
	defer stopSpinner()

	page := pagination.Resolve()
	pageSize := page.PageSize
	pageNumber := int64(1)

	resp, err := client.Teams.Members.GetTeamMembers(ctx, &pageSize, &pageNumber)
	if err != nil {
		stopSpinner()
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.TeamMembers == nil {
		stopSpinner()
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.TeamMembers.Data))
	result, err := pagination.Walk(resp, page,
		func(r *operations.GetTeamMembersResponse) func() (*operations.GetTeamMembersResponse, error) {
			return r.Next
		},
		func(r *operations.GetTeamMembersResponse, limit int) int {
			if r.TeamMembers == nil {
				return 0
			}
			data := r.TeamMembers.Data
			n := len(data)
			if limit >= 0 && n > limit {
				n = limit
			}
			for i := 0; i < n; i++ {
				rows = append(rows, teamMemberToRow(&data[i]))
			}
			return n
		},
	)
	if err != nil {
		stopSpinner()
		utils.PrintError(err)
		return nil
	}
	stopSpinner()
	renderer.Render(rows)
	if page.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}
	return nil
}

var teamMemberRoleNames = []string{"owner", "administrator", "collaborator", "billing"}

var teamMemberRoleDescriptions = []string{
	"Full control, including billing and team deletion",
	"Manage resources and members",
	"Manage resources",
	"Billing access only",
}

func runTeamMembersAdd(cmd *cobra.Command, _ []string) error {
	email, _ := cmd.Flags().GetString("email")
	roleStr, _ := cmd.Flags().GetString("role")
	firstName, _ := cmd.Flags().GetString("first-name")
	lastName, _ := cmd.Flags().GetString("last-name")

	// Missing required input: fall back to an interactive form when
	// possible. A missing email opens the full form (role and optional
	// names included); a missing role alone only prompts the role.
	if email == "" || roleStr == "" {
		if !canPromptInteractively(cmd) {
			missing := "email"
			if email != "" {
				missing = "role"
			}
			utils.PrintError(requiredFlagError(missing))
			return nil
		}

		var err error
		fullForm := email == ""
		if fullForm {
			email, err = tui.RunTextInput("Email", "user@example.com")
			if err != nil || email == "" {
				utils.PrintError(requiredFlagError("email"))
				return nil
			}
		}
		if roleStr == "" {
			roleStr, err = tui.RunList("Role", teamMemberRoleNames, teamMemberRoleDescriptions)
			if err != nil {
				utils.PrintError(err)
				return nil
			}
		}
		if fullForm && firstName == "" {
			firstName, err = tui.RunTextInput("First name (enter to skip)", "")
			if err != nil {
				utils.PrintError(err)
				return nil
			}
		}
		if fullForm && lastName == "" {
			lastName, err = tui.RunTextInput("Last name (enter to skip)", "")
			if err != nil {
				utils.PrintError(err)
				return nil
			}
		}
	}

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

func runTeamMembersRemove(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	var userID string
	if len(args) > 0 {
		userID = args[0]
	} else {
		// No argument: pick the member interactively when possible.
		if !canPromptInteractively(cmd) {
			utils.PrintError(errors.New("user id argument is required (lsh teams members remove <user-id>, or run interactively without --no-input)"))
			return nil
		}

		selected, err := selectTeamMember(ctx, client)
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		confirmed, err := tui.RunConfirm(fmt.Sprintf("Remove %s from the team?", selected.email))
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		if !confirmed {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return nil
		}
		userID = selected.id
	}

	_, err := client.TeamMembers.Delete(ctx, userID)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	fmt.Fprintf(os.Stdout, "✅ Team member %s removed successfully\n", userID)
	return nil
}

type teamMemberChoice struct {
	id    string
	email string
}

// selectTeamMember lists the current team's members and lets the user pick
// one, returning its id. Emails identify the choice since the list selector
// returns the selected label.
func selectTeamMember(ctx context.Context, client *latitudeshgosdk.Latitudesh) (teamMemberChoice, error) {
	stopSpinner := tui.StartFetchSpinner("Fetching team members…")
	defer stopSpinner()

	// The member picker must always see every member, independent of the
	// output-pagination flags (--max-items / --no-paginate), so it walks all
	// pages itself using the default page size.
	pageSize := pagination.DefaultPageSize
	resp, err := client.Teams.Members.GetTeamMembers(ctx, &pageSize, nil)
	if err != nil {
		return teamMemberChoice{}, err
	}

	byEmail := make(map[string]teamMemberChoice)
	var emails, descriptions []string
	for resp != nil && resp.TeamMembers != nil {
		for i := range resp.TeamMembers.Data {
			m := &resp.TeamMembers.Data[i]
			if m.ID == nil || m.Attributes == nil || m.Attributes.Email == nil {
				continue
			}
			email := *m.Attributes.Email
			if _, dup := byEmail[email]; dup {
				continue
			}
			byEmail[email] = teamMemberChoice{id: *m.ID, email: email}
			emails = append(emails, email)
			role := ""
			if m.Attributes.Role != nil && m.Attributes.Role.Name != nil {
				role = *m.Attributes.Role.Name
			}
			descriptions = append(descriptions, role)
		}
		if resp.Next == nil {
			break
		}
		resp, err = resp.Next()
		if err != nil {
			return teamMemberChoice{}, err
		}
	}
	stopSpinner()

	if len(emails) == 0 {
		return teamMemberChoice{}, errors.New("no team members found")
	}

	choice, err := tui.RunList("Select the member to remove", emails, descriptions)
	if err != nil {
		return teamMemberChoice{}, err
	}
	return byEmail[choice], nil
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
