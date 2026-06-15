package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ResourceTableModel generalizes the servers table pattern: an interactive
// list where enter opens a details view of the selected row.
type ResourceTableModel struct {
	table        table.Model
	totalRecords int
	title        string
	noun         string
	quitting     bool
	selected     int
	showDetails  bool
}

// NewResourceTable creates an interactive table with details navigation.
func NewResourceTable(title, noun string, columns []table.Column, rows []table.Row) ResourceTableModel {
	height := len(rows)
	if height > 25 {
		height = 25
	}
	if height < 10 {
		height = 10
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(PrimaryColor).
		BorderBottom(true).
		Bold(true).
		Foreground(PrimaryColor)

	s.Selected = selectedRowStyle
	s.Cell = s.Cell.Padding(0, 1)

	t.SetStyles(s)

	return ResourceTableModel{
		table:        t,
		totalRecords: len(rows),
		title:        title,
		noun:         noun,
	}
}

func (m ResourceTableModel) Init() tea.Cmd {
	return nil
}

func (m ResourceTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.selected = m.table.Cursor()
			m.showDetails = true
			m.quitting = true
			return m, tea.Quit
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m ResourceTableModel) View() string {
	if m.quitting {
		return ""
	}

	header := ""
	if m.title != "" {
		header = TitleStyle.Render(m.title) + "\n\n"
	}

	tableView := baseTableStyle.Render(m.table.View())

	footer := footerStyle.Render(
		fmt.Sprintf("Total: %d %s", m.totalRecords, m.noun),
	)

	help := HelpStyle.Render("↑/↓: navigate • enter: details • q/esc: quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tableView,
		"\n",
		footer,
		help,
	)
}

// SelectedIndex returns the selected index
func (m ResourceTableModel) SelectedIndex() int {
	return m.selected
}

// ShouldShowDetails returns if should show details
func (m ResourceTableModel) ShouldShowDetails() bool {
	return m.showDetails
}

// RunResourceTable runs an interactive table with enter-to-details
// navigation. detailTitleKey selects which field of the chosen row names
// the details view (e.g. "Address" → "IP Details: 192.0.2.10"); the
// details fields follow the table's column order.
func RunResourceTable(title, noun, detailTitlePrefix, detailTitleKey string, columns []table.Column, rows []table.Row, originals []map[string]string) error {
	if len(rows) == 0 {
		fmt.Println(ErrorStyle.Render("\nNo results found.\n"))
		return nil
	}

	fieldOrder := make([]string, 0, len(columns))
	for _, c := range columns {
		fieldOrder = append(fieldOrder, c.Title)
	}

	for {
		p := tea.NewProgram(
			NewResourceTable(title, noun, columns, rows),
			tea.WithAltScreen(),
		)

		m, err := p.Run()
		if err != nil {
			return err
		}

		if model, ok := m.(ResourceTableModel); ok {
			if model.ShouldShowDetails() && model.SelectedIndex() < len(originals) {
				selected := originals[model.SelectedIndex()]
				detailTitle := fmt.Sprintf("%s: %s", detailTitlePrefix, selected[detailTitleKey])
				if err := RunResourceDetails(detailTitle, selected, fieldOrder); err != nil {
					return err
				}

				continue
			}
		}

		break
	}

	return nil
}
