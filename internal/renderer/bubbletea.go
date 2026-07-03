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

	// Check if this is server data to use specialized table
	if isServerData(data) {
		renderServersWithDetails(data)
		return
	}

	if isIPData(data) {
		renderIPsWithDetails(data)
		return
	}

	if _, ok := data[0].(DetailFielder); ok {
		renderDetailFielders(data)
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

// DetailFielder lets a ResponseData expose extra key/value pairs for the
// interactive details view, beyond the compact TableRow columns.
type DetailFielder interface {
	DetailFields() map[string]string
}

// DetailView describes how a DetailFielder resource is titled in the
// interactive views, and in which order its detail-only fields appear after
// the table columns.
type DetailView struct {
	Title        string   // table title, e.g. "User Data"
	Noun         string   // footer noun, e.g. "entries"
	DetailPrefix string   // details title prefix, e.g. "User Data"
	TitleKey     string   // field naming the details view, e.g. "ID"
	FieldOrder   []string // detail-only fields, in display order
}

// DetailViewer optionally lets a DetailFielder describe its own view labels.
type DetailViewer interface {
	DetailView() DetailView
}

// resolveDetailView returns the item's self-described view labels, or a
// generic default.
func resolveDetailView(item ResponseData) DetailView {
	if v, ok := item.(DetailViewer); ok {
		return v.DetailView()
	}
	return DetailView{Title: "Results", Noun: "items", DetailPrefix: "Details", TitleKey: "ID"}
}

// detailFieldsFor flattens an item's table row plus its extra detail fields
// into the label/value map consumed by the details sheet.
func detailFieldsFor(item ResponseData) map[string]string {
	fields := make(map[string]string)
	for _, cell := range item.TableRow() {
		fields[cell.Label] = fmt.Sprintf("%v", cell.Value)
	}
	if df, ok := item.(DetailFielder); ok {
		for k, v := range df.DetailFields() {
			fields[k] = v
		}
	}
	return fields
}

// RenderDetails renders a single resource opening the details sheet directly
// in the interactive view (the `get` case). Structured formats (-o json/yaml/
// csv) and non-TTY runs fall back to the regular renderer.
func RenderDetails(data []ResponseData) {
	if _, interactive := GetRenderer().(BubbleTeaRenderer); interactive && len(data) == 1 {
		if _, ok := data[0].(DetailFielder); ok {
			view := resolveDetailView(data[0])
			fields := detailFieldsFor(data[0])

			columns, _ := convertToTableFormat(data)
			fieldOrder := make([]string, 0, len(columns)+len(view.FieldOrder))
			for _, c := range columns {
				fieldOrder = append(fieldOrder, c.Title)
			}
			fieldOrder = append(fieldOrder, view.FieldOrder...)

			title := fmt.Sprintf("%s: %s", view.DetailPrefix, fields[view.TitleKey])
			if err := tui.RunResourceDetails(title, fields, fieldOrder); err != nil {
				fmt.Printf("Error rendering details: %v\n", err)
			}
			return
		}
	}
	Render(data)
}

// renderDetailFielders renders lists of resources that expose rich detail
// fields: an enter-to-details table, even for a single row, so `list` output
// stays a table regardless of result count.
func renderDetailFielders(data []ResponseData) {
	view := resolveDetailView(data[0])

	columns, rows := convertToTableFormat(data)

	fieldOrder := make([]string, 0, len(columns)+len(view.FieldOrder))
	for _, c := range columns {
		fieldOrder = append(fieldOrder, c.Title)
	}
	fieldOrder = append(fieldOrder, view.FieldOrder...)

	originals := make([]map[string]string, 0, len(data))
	for _, item := range data {
		originals = append(originals, detailFieldsFor(item))
	}

	err := tui.RunResourceTableOrdered(view.Title, view.Noun, view.DetailPrefix, view.TitleKey, columns, rows, originals, fieldOrder)
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
