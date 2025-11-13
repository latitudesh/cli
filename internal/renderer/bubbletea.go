package renderer

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	"github.com/latitudesh/lsh/internal/tui"
)

// BubbleTeaRenderer renders ResponseData using Bubble Tea
type BubbleTeaRenderer struct{}

func (btr BubbleTeaRenderer) Render(data []ResponseData) {
	if len(data) == 0 {
		fmt.Println(tui.ErrorStyle.Render("\n✗ No results found.\n"))
		return
	}

	// Convert ResponseData to Bubble Tea format
	columns, rows := convertToTableFormat(data)

	// Render interactive table using Bubble Tea
	err := tui.RunInteractiveTable("Results", columns, rows)
	if err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
}

// preferredColumnOrder defines the preferred order of columns
var preferredColumnOrder = []string{
	"id",
	"name",
	"slug",
	"environment",
	"description",
	"provisioning_type",
	"team",
	"ips",
	"servers",
	"vlans",
	"tags",
}

// sortColumnsByPreference ordena as colunas baseado na ordem preferida
func sortColumnsByPreference(columnIDs []string) {
	// Create a map of priorities
	priority := make(map[string]int)
	for i, id := range preferredColumnOrder {
		priority[id] = i
	}

	// Sort using the priority
	sort.Slice(columnIDs, func(i, j int) bool {
		priI, okI := priority[columnIDs[i]]
		priJ, okJ := priority[columnIDs[j]]

		// If both are in the priority list, use the defined order
		if okI && okJ {
			return priI < priJ
		}

		// If only i is in the list, i comes first
		if okI {
			return true
		}

		// If only j is in the list, j comes first
		if okJ {
			return false
		}

		// If neither is in the list, alphabetical order
		return columnIDs[i] < columnIDs[j]
	})
}

// convertToTableFormat converts ResponseData to Bubble Tea format
func convertToTableFormat(data []ResponseData) ([]table.Column, []table.Row) {
	if len(data) == 0 {
		return nil, nil
	}

	// Extract headers from the first item
	firstRow := data[0].TableRow()
	
	var columnIDs []string
	columnWidths := make(map[string]int)

	// First pass: collect IDs and calculate minimum width of headers
	for id, cell := range firstRow {
		columnIDs = append(columnIDs, id)
		columnWidths[id] = len(cell.Label) + 2 // +2 for padding
	}

	// Sort columns by the preferred order
	sortColumnsByPreference(columnIDs)

	// Second pass: calculate maximum width based on real content
	for _, item := range data {
		row := item.TableRow()
		for id, cell := range row {
			value := fmt.Sprintf("%v", cell.Value)
			contentLen := len(value)
			
			// Update if this value is larger
			if contentLen > columnWidths[id] {
				columnWidths[id] = contentLen + 2 // +2 for padding
			}
		}
	}

	// Third pass: apply sensible limits and build Bubble Tea columns
	var columns []table.Column
	for _, id := range columnIDs {
		width := columnWidths[id]
		
		// Limits: minimum 10, maximum 50
		if width < 10 {
			width = 10
		}
		if width > 50 {
			width = 50 // Still truncate very large values
		}

		columns = append(columns, table.Column{
			Title: firstRow[id].Label,
			Width: width,
		})
	}

	// Build Bubble Tea rows without truncating (or truncate less)
	var rows []table.Row
	for _, item := range data {
		row := item.TableRow()
		var rowData table.Row

		for _, id := range columnIDs {
			cell := row[id]
			value := fmt.Sprintf("%v", cell.Value)
			
			// Only truncate if really necessary
			if len(value) > 50 {
				value = value[:47] + "..."
			}
			
			rowData = append(rowData, value)
		}
		
		rows = append(rows, rowData)
	}

	return columns, rows
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
