package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseTableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)
	
	selectedRowStyle = lipgloss.NewStyle().
    			Foreground(lipgloss.Color("0")). // Preto (ANSI)
    			Background(lipgloss.Color("14")).
				Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true).
			MarginTop(1)
)

type TableModel struct {
	table        table.Model
	totalRecords int
	title        string
	quitting     bool
}

// NewInteractiveTable is a helper function to create an interactive table
func NewInteractiveTable(title string, columns []table.Column, rows []table.Row) TableModel {
	// Adjust height based on the number of rows (max 20)
	height := len(rows)
	if height > 20 {
		height = 20
	}
	if height < 5 {
		height = 5
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(PrimaryColor).
		BorderBottom(true).
		Bold(true).
		Foreground(PrimaryColor)
	
	s.Selected = selectedRowStyle
	
	s.Cell = s.Cell.
		Padding(0, 1)

	t.SetStyles(s)

	return TableModel{
		table:        t,
		totalRecords: len(rows),
		title:        title,
	}
}

func (m TableModel) Init() tea.Cmd {
	return nil
}

func (m TableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			// You can add action to select a row
			// For now, quit
			m.quitting = true
			return m, tea.Quit
		}
	}
	
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m TableModel) View() string {
	if m.quitting {
		return ""
	}

	// Header
	header := ""
	if m.title != "" {
		header = TitleStyle.Render(m.title) + "\n\n"
	}

	// Tabela
	tableView := baseTableStyle.Render(m.table.View())

	// Footer with info and help
	footer := footerStyle.Render(
		fmt.Sprintf("Showing %d records", m.totalRecords),
	)
	
	help := HelpStyle.Render("↑/↓: navigate • q/esc: quit • enter: select")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tableView,
		"\n",
		footer,
		help,
	)
}

// SelectedRow returns the index of the selected row
func (m TableModel) SelectedRow() int {
	return m.table.Cursor()
}

// RunInteractiveTable runs an interactive table
func RunInteractiveTable(title string, columns []table.Column, rows []table.Row) error {
	if len(rows) == 0 {
		fmt.Println(ErrorStyle.Render("\nNo results found."))
		return nil
	}

	p := tea.NewProgram(
		NewInteractiveTable(title, columns, rows),
		tea.WithAltScreen(), // Use alternative screen
	)
	
	_, err := p.Run()
	return err
}
