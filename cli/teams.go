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

func makeOperationGroupTeamsCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "teams",
		Short: "Manage teams and team members",
	}

	listCmd, err := makeOperationTeamsListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	createCmd, err := makeOperationTeamsCreateCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(createCmd)

	updateCmd, err := makeOperationTeamsUpdateCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(updateCmd)

	membersCmd, err := makeOperationGroupTeamMembersCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(membersCmd)

	return cmd, nil
}

func makeOperationTeamsListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List every team you belong to",
		Example:      `  lsh teams list`,
		Args:         cobra.NoArgs,
		RunE:         runTeamsList,
		SilenceUsage: true,
	}
	return cmd, nil
}

func makeOperationTeamsCreateCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new team",
		Example: `  lsh teams create --name="My Team" --currency=USD
  lsh teams create --name="BR Team" --currency=BRL --address="Rua X, 100"`,
		Args:         cobra.NoArgs,
		RunE:         runTeamsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Team name (required)")
	cmd.Flags().String("currency", "USD", "Billing currency: USD or BRL")
	cmd.Flags().String("address", "", "Billing address")
	cmd.Flags().String("referred-code", "", "Referral code (first team only)")
	_ = cmd.MarkFlagRequired("name")

	return cmd, nil
}

func makeOperationTeamsUpdateCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "update <team-id>",
		Short: "Update a team",
		Example: `  lsh teams update tm_xxx --name="New Name"
  lsh teams update tm_xxx --currency=BRL`,
		Args:         cobra.ExactArgs(1),
		RunE:         runTeamsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New team name")
	cmd.Flags().String("currency", "", "Billing currency: USD or BRL")
	cmd.Flags().String("address", "", "Billing address")

	return cmd, nil
}

type teamRow struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Slug     string `json:"slug,omitempty"`
	Currency string `json:"currency,omitempty"`
	Owner    string `json:"owner,omitempty"`
}

func (t teamRow) TableRow() table.Row {
	return table.Row{
		"id":       {Value: t.ID, Label: "ID"},
		"name":     {Value: t.Name, Label: "Name"},
		"slug":     {Value: t.Slug, Label: "Slug"},
		"currency": {Value: t.Currency, Label: "Currency"},
		"owner":    {Value: t.Owner, Label: "Owner"},
	}
}

func runTeamsList(_ *cobra.Command, _ []string) error {
	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.UserProfile.ListTeams(ctx)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip rendering.")
		return nil
	}

	if resp == nil || resp.UserTeams == nil {
		renderer.Render(nil)
		return nil
	}

	rows := make([]renderer.ResponseData, 0, len(resp.UserTeams.Data))
	for i := range resp.UserTeams.Data {
		rows = append(rows, userTeamToRow(&resp.UserTeams.Data[i]))
	}
	renderer.Render(rows)
	return nil
}

func runTeamsCreate(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	currency, _ := cmd.Flags().GetString("currency")
	address, _ := cmd.Flags().GetString("address")
	referredCode, _ := cmd.Flags().GetString("referred-code")

	cur, err := parsePostTeamCurrency(currency)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	body := operations.PostTeamTeamsRequestBody{
		Data: operations.PostTeamTeamsData{
			Type: operations.PostTeamTeamsTypeTeams,
			Attributes: &operations.PostTeamTeamsAttributes{
				Name:     name,
				Currency: cur,
			},
		},
	}
	if address != "" {
		body.Data.Attributes.Address = &address
	}
	if referredCode != "" {
		body.Data.Attributes.ReferredCode = &referredCode
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.Teams.Create(ctx, body)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.Object == nil || resp.Object.Data == nil {
		renderer.Render(nil)
		return nil
	}
	renderer.Render([]renderer.ResponseData{teamToRow(resp.Object.Data)})
	return nil
}

func runTeamsUpdate(cmd *cobra.Command, args []string) error {
	teamID := args[0]
	name, _ := cmd.Flags().GetString("name")
	currency, _ := cmd.Flags().GetString("currency")
	address, _ := cmd.Flags().GetString("address")

	attrs := &operations.PatchCurrentTeamTeamsAttributes{}
	if name != "" {
		attrs.Name = &name
	}
	if address != "" {
		attrs.Address = &address
	}
	if currency != "" {
		cur, err := parsePatchTeamCurrency(currency)
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		attrs.Currency = &cur
	}

	body := operations.PatchCurrentTeamTeamsRequestBody{
		Data: operations.PatchCurrentTeamTeamsData{
			ID:         teamID,
			Type:       operations.PatchCurrentTeamTeamsTypeTeams,
			Attributes: attrs,
		},
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.Teams.Update(ctx, teamID, body)
	if err != nil {
		utils.PrintError(err)
		return nil
	}
	if resp == nil || resp.Object == nil || resp.Object.Data == nil {
		renderer.Render(nil)
		return nil
	}
	renderer.Render([]renderer.ResponseData{teamToRow(resp.Object.Data)})
	return nil
}

func parsePostTeamCurrency(s string) (operations.PostTeamCurrency, error) {
	switch strings.ToUpper(s) {
	case "USD":
		return operations.PostTeamCurrencyUsd, nil
	case "BRL":
		return operations.PostTeamCurrencyBrl, nil
	default:
		return "", &invalidEnumError{field: "currency", value: s, allowed: "USD, BRL"}
	}
}

func parsePatchTeamCurrency(s string) (operations.PatchCurrentTeamTeamsCurrency, error) {
	switch strings.ToUpper(s) {
	case "USD":
		return operations.PatchCurrentTeamTeamsCurrencyUsd, nil
	case "BRL":
		return operations.PatchCurrentTeamTeamsCurrencyBrl, nil
	default:
		return "", &invalidEnumError{field: "currency", value: s, allowed: "USD, BRL"}
	}
}

type invalidEnumError struct {
	field, value, allowed string
}

func (e *invalidEnumError) Error() string {
	return "invalid value for --" + e.field + ": '" + e.value + "' (allowed: " + e.allowed + ")"
}

func userTeamToRow(t *components.UserTeam) teamRow {
	row := teamRow{}
	if t == nil {
		return row
	}
	if t.ID != nil {
		row.ID = *t.ID
	}
	if t.Attributes == nil {
		return row
	}
	if t.Attributes.Name != nil {
		row.Name = *t.Attributes.Name
	}
	if t.Attributes.Slug != nil {
		row.Slug = *t.Attributes.Slug
	}
	if t.Attributes.Currency != nil {
		row.Currency = *t.Attributes.Currency
	}
	if t.Attributes.Owner != nil && t.Attributes.Owner.Email != nil {
		row.Owner = *t.Attributes.Owner.Email
	}
	return row
}

func teamToRow(t *components.Team) teamRow {
	row := teamRow{}
	if t == nil {
		return row
	}
	if t.ID != nil {
		row.ID = *t.ID
	}
	if t.Attributes == nil {
		return row
	}
	if t.Attributes.Name != nil {
		row.Name = *t.Attributes.Name
	}
	if t.Attributes.Slug != nil {
		row.Slug = *t.Attributes.Slug
	}
	if t.Attributes.Currency != nil {
		row.Currency = *t.Attributes.Currency
	}
	if t.Attributes.Owner != nil && t.Attributes.Owner.Email != nil {
		row.Owner = *t.Attributes.Owner.Email
	}
	return row
}
