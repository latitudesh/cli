package table

import (
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
)

const MaxColWidth = 18
const MaxLength = 120

type Table struct {
	Headers []string
	Rows    [][]string
}

type Header struct {
	IDs    []string
	Labels []string
}

func Render(rows []Row) {
	headers := extractHeaders(rows[0])

	tableWriter := tablewriter.NewWriter(os.Stdout)

	tableWriter.SetRowLine(true)
	tableWriter.SetHeader(headers.Labels)
	tableWriter.SetColWidth(MaxColWidth)

	for _, row := range rows {
		var tr []string

		for _, attr := range headers.IDs {
			tr = append(tr, renderCell(row[attr]))
		}

		tableWriter.Append(tr)
	}

	fmt.Printf("\n")
	tableWriter.Render()
	fmt.Printf("\n")
}

func RenderEmptyState(message string) {
	table := tablewriter.NewWriter(os.Stdout)

	table.SetRowLine(true)
	table.Append([]string{message})

	fmt.Println()
	table.Render()
	fmt.Println()
}

// preferredColumnOrder mirrors the interactive table ordering so static and
// Bubble Tea views print columns consistently. Unlisted columns follow in
// alphabetical order.
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
	"status",
	"phase",
	"ready",
	"plan",
	"os",
	"operating_system",
	"primary_ipv4",
	"region",
	"endpoint",
	"storage_class",
	"created_at",
	"updated_at",
}

func extractHeaders(row Row) Header {
	ids := make([]string, 0, len(row))
	for k := range row {
		ids = append(ids, k)
	}

	priority := make(map[string]int, len(preferredColumnOrder))
	for i, id := range preferredColumnOrder {
		priority[id] = i
	}
	sort.Slice(ids, func(i, j int) bool {
		pi, oki := priority[ids[i]]
		pj, okj := priority[ids[j]]
		switch {
		case oki && okj:
			return pi < pj
		case oki:
			return true
		case okj:
			return false
		default:
			return ids[i] < ids[j]
		}
	})

	var headers Header
	for _, k := range ids {
		headers.IDs = append(headers.IDs, k)
		headers.Labels = append(headers.Labels, row[k].Label)
	}

	return headers
}

func renderCell(cell Cell) string {
	maxLength := MaxLength

	if cell.MaxLength > 0 {
		maxLength = cell.MaxLength
	}

	return fmt.Sprintf("%v", truncate(cell.Value, maxLength))
}

func truncate(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength] + "..."
	}
	return fmt.Sprintf("%v", input)
}
