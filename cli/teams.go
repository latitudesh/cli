package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	cmd.Flags().String("name", "", "Team name (prompted interactively when omitted)")
	cmd.Flags().String("currency", "USD", "Billing currency: USD or BRL")
	cmd.Flags().String("address", "", "Billing address")
	cmd.Flags().String("referred-code", "", "Referral code (first team only)")

	return cmd, nil
}

func makeOperationTeamsUpdateCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "update [team-id]",
		Short: "Update a team",
		Example: `  lsh teams update tm_xxx --name="New Name"
  lsh teams update tm_xxx --address="Rua X, 100"
  lsh teams update   # pick the team and fields interactively`,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runTeamsUpdate,
		SilenceUsage: true,
	}

	// currency is intentionally absent: the API only allows writing it
	// on team creation (resource marks it writable: :create_action?).
	cmd.Flags().String("name", "", "New team name")
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
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	stopSpinner := tui.StartFetchSpinner("Fetching teams…")
	defer stopSpinner()

	// Unlike the other list commands, /user/teams is not paginated:
	// the SDK's ListTeams takes no page params and its response has no
	// Next(), so a single call already returns every team.
	resp, err := client.UserProfile.ListTeams(ctx)
	stopSpinner()
	if err != nil {
		utils.PrintError(err)
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

	// No --name: fall back to an interactive form when possible.
	if name == "" {
		if !canPromptInteractively(cmd) {
			utils.PrintError(requiredFlagError("name"))
			return nil
		}

		var err error
		name, err = tui.RunTextInput("Team name", "My Team")
		if err != nil || name == "" {
			utils.PrintError(requiredFlagError("name"))
			return nil
		}
		if !cmd.Flags().Changed("currency") {
			currency, err = tui.RunList("Billing currency",
				[]string{"USD", "BRL"},
				[]string{"US Dollar", "Brazilian Real"})
			if err != nil {
				utils.PrintError(err)
				return nil
			}
		}
		if address == "" {
			address, err = tui.RunTextInput("Billing address (enter to skip)", "")
			if err != nil {
				utils.PrintError(err)
				return nil
			}
		}
	}

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
	name, _ := cmd.Flags().GetString("name")
	address, _ := cmd.Flags().GetString("address")

	// The API only updates the team the token belongs to (auth tokens are
	// team-scoped; other ids return 404), so the team is resolved from the
	// active profile instead of prompting for a choice.
	currentID, currentLabel := currentTeamFromProfile(cmd)

	var teamID string
	if len(args) > 0 {
		teamID = args[0]
		if currentID != "" && teamID != currentID {
			utils.PrintError(teamMismatchError(currentID, teamID))
			return nil
		}
	} else {
		if currentID == "" {
			utils.PrintError(errors.New("team id argument is required (lsh teams update <team-id>)"))
			return nil
		}
		teamID = currentID
		if currentLabel != "" {
			fmt.Fprintf(os.Stdout, "Updating team %s (%s)\n", currentLabel, teamID)
			fmt.Fprintln(os.Stdout, tui.HelpStyle.Render("to update a different team, switch with `lsh profile use <profile>` or run `lsh login`"))
		}
	}

	// No flags: fall back to an interactive form when possible, where
	// pressing enter on an empty field keeps the current value.
	if name == "" && address == "" {
		if !canPromptInteractively(cmd) {
			utils.PrintError(errors.New("nothing to update: pass at least one of --name or --address"))
			return nil
		}

		var err error
		name, err = tui.RunTextInput("New name (enter to keep current)", "")
		if err != nil {
			utils.PrintError(err)
			return nil
		}
		address, err = tui.RunTextInput("New billing address (enter to keep current)", "")
		if err != nil {
			utils.PrintError(err)
			return nil
		}

		if name == "" && address == "" {
			utils.PrintError(errors.New("nothing to update: every field was left unchanged"))
			return nil
		}
	}

	attrs := &operations.PatchCurrentTeamTeamsAttributes{}
	if name != "" {
		attrs.Name = &name
	}
	if address != "" {
		attrs.Address = &address
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

	// The SDK injects `"currency":"USD"` into every team PATCH because the
	// published spec declares a default for it, but the API only allows
	// writing currency on create — every update would 400 with
	// "unwritable_attribute". Strip it from the wire until the spec drops
	// currency from the PATCH body and the SDK is regenerated.
	httpClient := &http.Client{Transport: stripTeamPatchCurrency{base: http.DefaultTransport}}
	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity(viper.GetString("Authorization")),
		latitudeshgosdk.WithClient(httpClient),
	)
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
	refreshProfileTeam(cmd, resp.Object.Data)
	renderer.Render([]renderer.ResponseData{teamToRow(resp.Object.Data)})
	return nil
}

// teamMismatchError explains why a team other than the session's cannot be
// updated, with the shortest path to fix it: switch profiles when one
// exists for the target team, log in when the team is real but has no
// profile, or flag a likely typo when the id is not among the user's teams.
func teamMismatchError(currentID, teamID string) error {
	base := fmt.Sprintf("tokens are tied to a single team: this session can only update %s", currentID)

	if profileName := profileNameForTeam(teamID); profileName != "" {
		return fmt.Errorf("%s; to update %s, switch to it with `lsh profile use %s`", base, teamID, profileName)
	}

	// No profile for the target: check it is actually one of the user's
	// teams before suggesting a login — an unknown id is most likely a typo.
	known, checked := isUserTeam(teamID)
	if checked && !known {
		return fmt.Errorf("%s; %s is not one of your teams — check the id with `lsh teams list`", base, teamID)
	}
	return fmt.Errorf("%s; to update %s, run `lsh login` and pick that team first", base, teamID)
}

// isUserTeam reports whether the id belongs to one of the user's teams.
// checked is false when the listing failed and nothing can be asserted.
func isUserTeam(teamID string) (known, checked bool) {
	client := lsh.NewClient()
	resp, err := client.UserProfile.ListTeams(context.Background())
	if err != nil || resp == nil || resp.UserTeams == nil {
		return false, false
	}
	for i := range resp.UserTeams.Data {
		if id := resp.UserTeams.Data[i].ID; id != nil && *id == teamID {
			return true, true
		}
	}
	return false, true
}

// profileNameForTeam returns the name of a stored profile bound to the
// given team id, or "" when none exists. Used to suggest `lsh profile use`
// instead of a fresh login when the user already has a session for the
// team they are targeting.
func profileNameForTeam(teamID string) string {
	f, err := config.Load()
	if err != nil {
		return ""
	}
	for _, name := range f.SortedProfileNames() {
		if f.Profiles[name].TeamID == teamID {
			return name
		}
	}
	return ""
}

// refreshProfileTeam syncs the stored profile's team metadata after a
// successful update, so `profile list` and prompts show the new name
// instead of the one captured at login time.
func refreshProfileTeam(cmd *cobra.Command, t *components.Team) {
	if t == nil || t.ID == nil {
		return
	}
	f, err := config.Load()
	if err != nil {
		return
	}
	override, _ := cmd.Flags().GetString("profile")
	name, p, err := f.Resolve(override)
	if err != nil || p.TeamID != *t.ID || t.Attributes == nil {
		return
	}

	changed := false
	if t.Attributes.Name != nil && *t.Attributes.Name != p.TeamName {
		p.TeamName = *t.Attributes.Name
		changed = true
	}
	if t.Attributes.Slug != nil && *t.Attributes.Slug != p.TeamSlug {
		p.TeamSlug = *t.Attributes.Slug
		changed = true
	}
	if !changed {
		return
	}
	f.SetProfile(name, p)
	if err := config.Save(f); err != nil {
		lsh.LogDebugf("could not refresh profile team metadata: %v", err)
	}
}

// currentTeamFromProfile resolves the team tied to the active profile,
// returning zero values when no profile/team is configured (e.g. raw
// token auth without a stored profile).
func currentTeamFromProfile(cmd *cobra.Command) (id, label string) {
	f, err := config.Load()
	if err != nil {
		return "", ""
	}
	override, _ := cmd.Flags().GetString("profile")
	_, p, err := f.Resolve(override)
	if err != nil {
		return "", ""
	}
	label = p.TeamName
	if label == "" {
		label = p.TeamSlug
	}
	return p.TeamID, label
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

// stripTeamPatchCurrency removes the unwritable currency attribute that the
// SDK adds to team PATCH bodies via the spec's default value.
type stripTeamPatchCurrency struct {
	base http.RoundTripper
}

func (t stripTeamPatchCurrency) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPatch && req.Body != nil {
		raw, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		if stripped, ok := removeBodyAttribute(raw, "currency"); ok {
			raw = stripped
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		req.ContentLength = int64(len(raw))
	}
	return t.base.RoundTrip(req)
}

// removeBodyAttribute deletes data.attributes.<name> from a JSON:API body,
// reporting whether the body was rewritten.
func removeBodyAttribute(raw []byte, name string) ([]byte, bool) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, false
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		return nil, false
	}
	if _, present := attrs[name]; !present {
		return nil, false
	}
	delete(attrs, name)
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return rewritten, true
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
