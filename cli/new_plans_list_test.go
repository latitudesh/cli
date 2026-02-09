package cli

import (
	"encoding/json"
	"testing"
)

func makePlansResponse() *plansResponse {
	const plansResponseFixture = `{
		"data": [
			{
				"id": "plan_123",
				"attributes": {
					"slug": "m4-metal-medium",
					"specs": {
						"cpu": {
							"count": 1,
							"cores": 16,
							"type": "AMD 9124"
						},
						"memory": {
							"total": 128
						}
					},
					"regions": [
						{
							"name": "United States",
							"locations": {
								"available": ["LAX", "LAX2", "ASH", "CHI"],
								"in_stock": ["LAX", "ASH", "CHI"]
							},
							"pricing": {
								"USD": {
									"month": "455.00"
								}
							},
							"stock_level": "high"
						},
						{
							"name": "Brazil",
							"locations": {
								"available": ["SAO", "SAO2"],
								"in_stock": []
							},
							"pricing": {
								"USD": {
									"month": "577.00"
								}
							},
							"stock_level": "unavailable"
						}
					]
				}
			}
		]
	}`

	pr := &plansResponse{}
	if err := json.Unmarshal([]byte(plansResponseFixture), pr); err != nil {
		// In tests, a panic on invalid fixture data is acceptable.
		panic(err)
	}
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

	// Assert exact set: only LAX, ASH, CHI (the in_stock locations from US region)
	expected := map[string]bool{"LAX": true, "ASH": true, "CHI": true}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Location] = true
	}

	if len(got) != len(expected) {
		t.Errorf("--available returned %d locations %v, want %d locations %v", len(got), got, len(expected), expected)
	}
	for loc := range expected {
		if !got[loc] {
			t.Errorf("expected %s to be included with --available", loc)
		}
	}
	for loc := range got {
		if !expected[loc] {
			t.Errorf("unexpected location %s included with --available", loc)
		}
	}
}

func TestFilterAndFlatten_StockLevelFilter(t *testing.T) {
	pr := makePlansResponse()

	// Filter by stock_level=high: should only include locations that have stock_level "high"
	rows := filterAndFlatten(pr, "", "", false, false, false, "", "", "high", 0, 0, 0, 0, 0, 0)

	// Assert exact set: only LAX, ASH, CHI (in_stock locations in US region with "high")
	expected := map[string]bool{"LAX": true, "ASH": true, "CHI": true}
	got := map[string]bool{}
	for _, r := range rows {
		if r.StockLevel != "high" {
			t.Errorf("stock_level filter 'high' returned location %s with stock_level %q", r.Location, r.StockLevel)
		}
		got[r.Location] = true
	}

	if len(got) != len(expected) {
		t.Errorf("stock_level=high returned %d locations %v, want %d locations %v", len(got), got, len(expected), expected)
	}
	for loc := range expected {
		if !got[loc] {
			t.Errorf("expected %s to be included with stock_level=high", loc)
		}
	}
	for loc := range got {
		if !expected[loc] {
			t.Errorf("unexpected location %s included with stock_level=high", loc)
		}
	}
}

func TestFilterAndFlatten_StockLevelFilterUnavailable(t *testing.T) {
	pr := makePlansResponse()

	// Filter by stock_level=unavailable: should return locations NOT in in_stock
	rows := filterAndFlatten(pr, "", "", false, false, false, "", "", "unavailable", 0, 0, 0, 0, 0, 0)

	// Assert exact set: LAX2 (US, not in_stock), SAO, SAO2 (Brazil, not in_stock)
	expected := map[string]bool{"LAX2": true, "SAO": true, "SAO2": true}
	got := map[string]bool{}
	for _, r := range rows {
		if r.StockLevel != "unavailable" {
			t.Errorf("stock_level filter 'unavailable' returned location %s with stock_level %q", r.Location, r.StockLevel)
		}
		got[r.Location] = true
	}

	if len(got) != len(expected) {
		t.Errorf("stock_level=unavailable returned %d locations %v, want %d locations %v", len(got), got, len(expected), expected)
	}
	for loc := range expected {
		if !got[loc] {
			t.Errorf("expected %s to be included with stock_level=unavailable", loc)
		}
	}
	for loc := range got {
		if !expected[loc] {
			t.Errorf("unexpected location %s included with stock_level=unavailable", loc)
		}
	}
}

func TestFilterAndFlatten_InStockFlag(t *testing.T) {
	pr := makePlansResponse()

	// With --in_stock: should only show locations in the in_stock list
	rows := filterAndFlatten(pr, "", "", true, false, false, "", "", "", 0, 0, 0, 0, 0, 0)

	// Assert exact set: only LAX, ASH, CHI (the in_stock locations from US region)
	expected := map[string]bool{"LAX": true, "ASH": true, "CHI": true}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Location] = true
	}

	if len(got) != len(expected) {
		t.Errorf("--in_stock returned %d locations %v, want %d locations %v", len(got), got, len(expected), expected)
	}
	for loc := range expected {
		if !got[loc] {
			t.Errorf("expected %s to be included with --in_stock", loc)
		}
	}
	for loc := range got {
		if !expected[loc] {
			t.Errorf("unexpected location %s included with --in_stock", loc)
		}
	}
}
