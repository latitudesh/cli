package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
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

func TestParsePatchTeamCurrency(t *testing.T) {
	cases := []struct {
		in      string
		want    operations.PatchCurrentTeamTeamsCurrency
		wantErr bool
	}{
		{in: "USD", want: operations.PatchCurrentTeamTeamsCurrencyUsd},
		{in: "brl", want: operations.PatchCurrentTeamTeamsCurrencyBrl},
		{in: "GBP", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parsePatchTeamCurrency(tc.in)
		if tc.wantErr != (err != nil) {
			t.Errorf("parsePatchTeamCurrency(%q): err = %v, wantErr = %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("parsePatchTeamCurrency(%q) = %q, want %q", tc.in, got, tc.want)
		}
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
