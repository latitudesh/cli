package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSimpleTable renders a simple table (without interaction)
// Useful when you want to show results without blocking
func RenderSimpleTable(title string, headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Println(ErrorStyle.Render("\n✗ No results found.\n"))
		return
	}

	// Header
	if title != "" {
		fmt.Println(TitleStyle.Render(title))
		fmt.Println()
	}

	// Calculate widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Limit maximum width
	for i := range colWidths {
		if colWidths[i] > 30 {
			colWidths[i] = 30
		}
		colWidths[i] += 2 // padding
	}

	// Render headers
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1)

	var headerParts []string
	for i, h := range headers {
		headerParts = append(headerParts, padString(h, colWidths[i]))
	}
	fmt.Println(headerStyle.Render(strings.Join(headerParts, " │ ")))

	// Separator line
	var separators []string
	for _, width := range colWidths {
		separators = append(separators, strings.Repeat("─", width))
	}
	fmt.Println(lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Render(strings.Join(separators, "─┼─")))

	// Render rows
	rowStyle := lipgloss.NewStyle().Padding(0, 1)
	
	for _, row := range rows {
		var rowParts []string
		for i, cell := range row {
			// Truncate if necessary
			if len(cell) > colWidths[i]-2 {
				cell = cell[:colWidths[i]-5] + "..."
			}
			rowParts = append(rowParts, padString(cell, colWidths[i]))
		}
		fmt.Println(rowStyle.Render(strings.Join(rowParts, " │ ")))
	}

	// Footer
	fmt.Println()
	fmt.Println(footerStyle.Render(fmt.Sprintf("Total: %d records", len(rows))))
	fmt.Println()
}

func padString(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}
