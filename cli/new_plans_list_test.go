package cli

import (
	"testing"
)

func makePlansResponse() *plansResponse {
	pr := &plansResponse{}
	pr.Data = append(pr.Data, struct {
		ID         string `json:"id"`
		Attributes struct {
			Slug  string `json:"slug"`
			Specs struct {
				CPU struct {
					Count int            `json:"count"`
					Cores int            `json:"cores"`
					Clock flexibleString `json:"clock"`
					Type  string         `json:"type"`
				} `json:"cpu"`
				Memory struct {
					Total int `json:"total"`
				} `json:"memory"`
				Drives []struct {
					Count int    `json:"count"`
					Size  string `json:"size"`
					Type  string `json:"type"`
				} `json:"drives"`
			} `json:"specs"`
			Regions []struct {
				Name      string `json:"name"`
				Locations struct {
					Available []string `json:"available"`
					InStock   []string `json:"in_stock"`
				} `json:"locations"`
				Pricing struct {
					USD struct {
						Month flexibleString `json:"month"`
					} `json:"USD"`
				} `json:"pricing"`
				StockLevel string `json:"stock_level"`
			} `json:"regions"`
		} `json:"attributes"`
	}{
		ID: "plan_123",
	})

	d := &pr.Data[0]
	d.Attributes.Slug = "m4-metal-medium"
	d.Attributes.Specs.CPU.Count = 1
	d.Attributes.Specs.CPU.Cores = 16
	d.Attributes.Specs.CPU.Type = "AMD 9124"
	d.Attributes.Specs.Memory.Total = 128

	// United States region: LAX in stock, LAX2 NOT in stock
	d.Attributes.Regions = append(d.Attributes.Regions, struct {
		Name      string `json:"name"`
		Locations struct {
			Available []string `json:"available"`
			InStock   []string `json:"in_stock"`
		} `json:"locations"`
		Pricing struct {
			USD struct {
				Month flexibleString `json:"month"`
			} `json:"USD"`
		} `json:"pricing"`
		StockLevel string `json:"stock_level"`
	}{
		Name:       "United States",
		StockLevel: "high",
	})
	usRegion := &d.Attributes.Regions[0]
	usRegion.Locations.Available = []string{"LAX", "LAX2", "ASH", "CHI"}
	usRegion.Locations.InStock = []string{"LAX", "ASH", "CHI"}
	usRegion.Pricing.USD.Month = flexibleString{Value: "455.00"}

	// Brazil region: all unavailable
	d.Attributes.Regions = append(d.Attributes.Regions, struct {
		Name      string `json:"name"`
		Locations struct {
			Available []string `json:"available"`
			InStock   []string `json:"in_stock"`
		} `json:"locations"`
		Pricing struct {
			USD struct {
				Month flexibleString `json:"month"`
			} `json:"USD"`
		} `json:"pricing"`
		StockLevel string `json:"stock_level"`
	}{
		Name:       "Brazil",
		StockLevel: "unavailable",
	})
	brRegion := &d.Attributes.Regions[1]
	brRegion.Locations.Available = []string{"SAO", "SAO2"}
	brRegion.Locations.InStock = []string{}
	brRegion.Pricing.USD.Month = flexibleString{Value: "577.00"}

	return pr
}

func TestFilterAndFlatten_PerLocationStockLevel(t *testing.T) {
	pr := makePlansResponse()

	// No filters: should return all locations
	rows := filterAndFlatten(pr, "", "", false, false, false, "", "", "", 0, 0, 0, 0, 0, 0)

	// Find LAX and LAX2 rows
	var laxRow, lax2Row *flatRow
	for i := range rows {
		switch rows[i].Location {
		case "LAX":
			laxRow = &rows[i]
		case "LAX2":
			lax2Row = &rows[i]
		}
	}

	if laxRow == nil {
		t.Fatal("expected LAX row to exist")
	}
	if lax2Row == nil {
		t.Fatal("expected LAX2 row to exist")
	}

	// LAX is in in_stock -> should keep the region's stock level ("high")
	if laxRow.StockLevel != "high" {
		t.Errorf("LAX stock level = %q, want %q", laxRow.StockLevel, "high")
	}

	// LAX2 is NOT in in_stock -> should be "unavailable"
	if lax2Row.StockLevel != "unavailable" {
		t.Errorf("LAX2 stock level = %q, want %q", lax2Row.StockLevel, "unavailable")
	}
}

func TestFilterAndFlatten_AvailableFlag(t *testing.T) {
	pr := makePlansResponse()

	// With --available: should only show locations with stock
	rows := filterAndFlatten(pr, "", "", false, true, false, "", "", "", 0, 0, 0, 0, 0, 0)

	for _, r := range rows {
		if r.StockLevel == "unavailable" {
			t.Errorf("--available should not include location %s with stock_level %q", r.Location, r.StockLevel)
		}
	}

	// Should include LAX (in stock) but not LAX2 or SAO/SAO2 (not in stock)
	locations := map[string]bool{}
	for _, r := range rows {
		locations[r.Location] = true
	}

	if !locations["LAX"] {
		t.Error("expected LAX to be included with --available")
	}
	if locations["LAX2"] {
		t.Error("expected LAX2 to NOT be included with --available")
	}
	if locations["SAO"] {
		t.Error("expected SAO to NOT be included with --available")
	}
	if locations["SAO2"] {
		t.Error("expected SAO2 to NOT be included with --available")
	}
}

func TestFilterAndFlatten_StockLevelFilter(t *testing.T) {
	pr := makePlansResponse()

	// Filter by stock_level=high: should only include locations that have stock_level "high"
	rows := filterAndFlatten(pr, "", "", false, false, false, "", "", "high", 0, 0, 0, 0, 0, 0)

	for _, r := range rows {
		if r.StockLevel != "high" {
			t.Errorf("stock_level filter 'high' returned location %s with stock_level %q", r.Location, r.StockLevel)
		}
	}

	// Should include LAX, ASH, CHI (in stock in US region with "high")
	// Should NOT include LAX2 (not in in_stock, derived as "unavailable")
	locations := map[string]bool{}
	for _, r := range rows {
		locations[r.Location] = true
	}

	if !locations["LAX"] {
		t.Error("expected LAX to be included with stock_level=high")
	}
	if locations["LAX2"] {
		t.Error("expected LAX2 to NOT be included with stock_level=high")
	}
}

func TestFilterAndFlatten_StockLevelFilterUnavailable(t *testing.T) {
	pr := makePlansResponse()

	// Filter by stock_level=unavailable: should return locations NOT in in_stock
	rows := filterAndFlatten(pr, "", "", false, false, false, "", "", "unavailable", 0, 0, 0, 0, 0, 0)

	for _, r := range rows {
		if r.StockLevel != "unavailable" {
			t.Errorf("stock_level filter 'unavailable' returned location %s with stock_level %q", r.Location, r.StockLevel)
		}
	}

	locations := map[string]bool{}
	for _, r := range rows {
		locations[r.Location] = true
	}

	// LAX2 is in available but NOT in in_stock -> derived as "unavailable"
	if !locations["LAX2"] {
		t.Error("expected LAX2 to be included with stock_level=unavailable")
	}
	// SAO and SAO2 are in available but NOT in in_stock -> derived as "unavailable"
	if !locations["SAO"] {
		t.Error("expected SAO to be included with stock_level=unavailable")
	}
	if !locations["SAO2"] {
		t.Error("expected SAO2 to be included with stock_level=unavailable")
	}
	// LAX is in in_stock -> should NOT appear
	if locations["LAX"] {
		t.Error("expected LAX to NOT be included with stock_level=unavailable")
	}
}

func TestFilterAndFlatten_InStockFlag(t *testing.T) {
	pr := makePlansResponse()

	// With --in_stock: should only show locations in the in_stock list
	rows := filterAndFlatten(pr, "", "", true, false, false, "", "", "", 0, 0, 0, 0, 0, 0)

	for _, r := range rows {
		if r.StockLevel == "unavailable" {
			t.Errorf("--in_stock should not include location %s with stock_level %q", r.Location, r.StockLevel)
		}
	}

	locations := map[string]bool{}
	for _, r := range rows {
		locations[r.Location] = true
	}

	if !locations["LAX"] {
		t.Error("expected LAX to be included with --in_stock")
	}
	if locations["LAX2"] {
		t.Error("expected LAX2 to NOT be included with --in_stock")
	}
	if locations["SAO"] {
		t.Error("expected SAO to NOT be included with --in_stock")
	}
}
