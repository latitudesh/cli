package renderer

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	outputtable "github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/tui"
)

// BubbleTeaRenderer renders ResponseData using Bubble Tea
type BubbleTeaRenderer struct{}

func (btr BubbleTeaRenderer) Render(data []ResponseData) {
	if len(data) == 0 {
		fmt.Println(tui.ErrorStyle.Render("\n✗ No results found.\n"))
		return
	}

	// Check if this is server data to use specialized table
	if isServerData(data) {
		renderServersWithDetails(data)
		return
	}

	if isIPData(data) {
		renderIPsWithDetails(data)
		return
	}

	// A single plain row doesn't warrant a full-screen interactive table;
	// print it flat so the terminal isn't taken over for trivial output.
	if len(data) == 1 {
		GetStaticRenderer().Render(data)
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

// isServerData checks if the data is server data
func isServerData(data []ResponseData) bool {
	if len(data) == 0 {
		return false
	}

	firstRow := data[0].TableRow()
	// Check for server-specific fields
	_, hasHostname := firstRow["hostname"]
	_, hasIPMI := firstRow["ipmi_status"]
	return hasHostname && hasIPMI
}

// isIPData checks if the data is IP address data
func isIPData(data []ResponseData) bool {
	if len(data) == 0 {
		return false
	}

	firstRow := data[0].TableRow()
	_, hasAddress := firstRow["address"]
	_, hasFamily := firstRow["family"]
	return hasAddress && hasFamily
}

// renderIPsWithDetails renders IPs with enter-to-details support
func renderIPsWithDetails(data []ResponseData) {
	columns, rows := convertToTableFormat(data)

	var originals []map[string]string
	for _, item := range data {
		row := item.TableRow()
		fields := make(map[string]string)
		for _, cell := range row {
			fields[cell.Label] = fmt.Sprintf("%v", cell.Value)
		}
		originals = append(originals, fields)
	}

	err := tui.RunResourceTable("IP Addresses", "IPs", "IP Details", "Address", columns, rows, originals)
	if err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
}

// renderServersWithDetails renders servers with details support
func renderServersWithDetails(data []ResponseData) {
	columns, rows := convertToTableFormat(data)

	// Build original servers data for details view
	var originalServers []map[string]string
	for _, item := range data {
		row := item.TableRow()
		serverData := make(map[string]string)
		for _, cell := range row {
			serverData[cell.Label] = fmt.Sprintf("%v", cell.Value)
		}
		originalServers = append(originalServers, serverData)
	}

	err := tui.RunServersTable("Servers", columns, rows, originalServers)
	if err != nil {
		fmt.Printf("Error rendering table: %v\n", err)
	}
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
	outputtable.SortColumnsByPreference(columnIDs)

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
