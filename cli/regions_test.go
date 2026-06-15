package cli

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func TestRegionsDataToRow(t *testing.T) {
	const fixture = `{
		"id": "loc_123",
		"attributes": {
			"slug": "SAO",
			"name": "Sao Paulo",
			"country": {"slug": "BR", "name": "Brazil"}
		}
	}`
	region := &components.RegionsData{}
	if err := json.Unmarshal([]byte(fixture), region); err != nil {
		t.Fatal(err)
	}

	row := regionsDataToRow(region)
	want := regionRow{ID: "loc_123", Slug: "SAO", Name: "Sao Paulo", CountrySlug: "BR", CountryName: "Brazil"}
	if row != want {
		t.Errorf("regionsDataToRow = %+v, want %+v", row, want)
	}
}

func TestRegionsDataToRow_NilSafety(t *testing.T) {
	if row := regionsDataToRow(nil); row != (regionRow{}) {
		t.Errorf("regionsDataToRow(nil) = %+v, want zero row", row)
	}

	id := "loc_456"
	row := regionsDataToRow(&components.RegionsData{ID: &id})
	if row != (regionRow{ID: "loc_456"}) {
		t.Errorf("regionsDataToRow without attributes = %+v, want only ID set", row)
	}

	slug := "NYC"
	row = regionsDataToRow(&components.RegionsData{
		Attributes: &components.RegionsAttributes{Slug: &slug},
	})
	if row != (regionRow{Slug: "NYC"}) {
		t.Errorf("regionsDataToRow without country = %+v, want only Slug set", row)
	}
}
