package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/spf13/cobra"
)

func TestParsePostTeamCurrency(t *testing.T) {
	cases := []struct {
		in      string
		want    operations.PostTeamCurrency
		wantErr bool
	}{
		{in: "USD", want: operations.PostTeamCurrencyUsd},
		{in: "usd", want: operations.PostTeamCurrencyUsd},
		{in: "BRL", want: operations.PostTeamCurrencyBrl},
		{in: "brl", want: operations.PostTeamCurrencyBrl},
		{in: "EUR", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parsePostTeamCurrency(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parsePostTeamCurrency(%q): expected error, got %q", tc.in, got)
			} else if !strings.Contains(err.Error(), "USD, BRL") {
				t.Errorf("parsePostTeamCurrency(%q): error should list allowed values, got %q", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePostTeamCurrency(%q): unexpected error: %v", tc.in, err)
		} else if got != tc.want {
			t.Errorf("parsePostTeamCurrency(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRefreshProfileTeam(t *testing.T) {
	home := withTempHome(t)
	writeConfig(t, home, `{
		"default_profile": "work",
		"profiles": {
			"work": {"authorization": "k", "team_id": "team_1", "team_name": "Old Name", "team_slug": "old-name"}
		}
	}`)

	cmd := &cobra.Command{}
	cmd.Flags().String("profile", "", "")

	id := "team_1"
	newName := "New Name"
	newSlug := "new-name"
	refreshProfileTeam(cmd, &components.Team{
		ID:         &id,
		Attributes: &components.TeamAttributes{Name: &newName, Slug: &newSlug},
	})

	got := readConfig(t, home)
	if !strings.Contains(got, `"team_name": "New Name"`) && !strings.Contains(got, `"team_name":"New Name"`) {
		t.Errorf("profile team_name not refreshed, config: %s", got)
	}

	// A different team id must not touch the stored profile.
	otherID := "team_2"
	otherName := "Other"
	refreshProfileTeam(cmd, &components.Team{ID: &otherID, Attributes: &components.TeamAttributes{Name: &otherName}})
	got = readConfig(t, home)
	if strings.Contains(got, "Other") {
		t.Errorf("profile refreshed for a team it does not belong to: %s", got)
	}
}

func TestRemoveBodyAttribute(t *testing.T) {
	// The SDK injects the spec's default currency into every team PATCH
	// body; the transport must strip it without touching other fields.
	name := "New Name"
	body := operations.PatchCurrentTeamTeamsRequestBody{
		Data: operations.PatchCurrentTeamTeamsData{
			ID:         "tm_x",
			Type:       operations.PatchCurrentTeamTeamsTypeTeams,
			Attributes: &operations.PatchCurrentTeamTeamsAttributes{Name: &name},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"currency"`) {
		t.Fatalf("expected SDK to inject currency default, got %s", raw)
	}

	stripped, ok := removeBodyAttribute(raw, "currency")
	if !ok {
		t.Fatal("removeBodyAttribute: expected rewrite, got ok=false")
	}
	if strings.Contains(string(stripped), `"currency"`) {
		t.Errorf("currency still present after strip: %s", stripped)
	}
	if !strings.Contains(string(stripped), `"name":"New Name"`) {
		t.Errorf("name was lost during strip: %s", stripped)
	}

	// Bodies without the attribute are reported as untouched.
	if _, ok := removeBodyAttribute(stripped, "currency"); ok {
		t.Error("expected ok=false when attribute is absent")
	}
	if _, ok := removeBodyAttribute([]byte("not json"), "currency"); ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

func TestUserTeamToRow(t *testing.T) {
	const fixture = `{
		"id": "tm_123",
		"attributes": {
			"name": "My Team",
			"slug": "my-team",
			"currency": "USD",
			"owner": {"email": "owner@example.com"}
		}
	}`
	team := &components.UserTeam{}
	if err := json.Unmarshal([]byte(fixture), team); err != nil {
		t.Fatal(err)
	}

	row := userTeamToRow(team)
	want := teamRow{ID: "tm_123", Name: "My Team", Slug: "my-team", Currency: "USD", Owner: "owner@example.com"}
	if row != want {
		t.Errorf("userTeamToRow = %+v, want %+v", row, want)
	}
}

func TestUserTeamToRow_NilSafety(t *testing.T) {
	if row := userTeamToRow(nil); row != (teamRow{}) {
		t.Errorf("userTeamToRow(nil) = %+v, want zero row", row)
	}

	id := "tm_456"
	row := userTeamToRow(&components.UserTeam{ID: &id})
	if row != (teamRow{ID: "tm_456"}) {
		t.Errorf("userTeamToRow without attributes = %+v, want only ID set", row)
	}
}

func TestTeamToRow(t *testing.T) {
	const fixture = `{
		"id": "tm_789",
		"attributes": {
			"name": "New Team",
			"slug": "new-team",
			"currency": "BRL",
			"owner": {"email": "ceo@example.com"}
		}
	}`
	team := &components.Team{}
	if err := json.Unmarshal([]byte(fixture), team); err != nil {
		t.Fatal(err)
	}

	row := teamToRow(team)
	want := teamRow{ID: "tm_789", Name: "New Team", Slug: "new-team", Currency: "BRL", Owner: "ceo@example.com"}
	if row != want {
		t.Errorf("teamToRow = %+v, want %+v", row, want)
	}

	if got := teamToRow(nil); got != (teamRow{}) {
		t.Errorf("teamToRow(nil) = %+v, want zero row", got)
	}
}
