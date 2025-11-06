package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlansTableModel is a specialized table for plans
type PlansTableModel struct {
	table         table.Model
	totalRecords  int
	title         string
	quitting      bool
	selected      int
	showDetails   bool
	originalPlans []map[string]string
}

// NewPlansTable cria uma tabela interativa para plans
func NewPlansTable(title string, columns []table.Column, rows []table.Row, originalPlans []map[string]string) PlansTableModel {
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

	return PlansTableModel{
		table:         t,
		totalRecords:  len(rows),
		title:         title,
		originalPlans: originalPlans,
	}
}

func (m PlansTableModel) Init() tea.Cmd {
	return nil
}

func (m PlansTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m PlansTableModel) View() string {
	if m.quitting {
		return ""
	}

	header := ""
	if m.title != "" {
		header = TitleStyle.Render(m.title) + "\n\n"
	}

	tableView := baseTableStyle.Render(m.table.View())

	footer := footerStyle.Render(
		fmt.Sprintf("Total: %d plans", m.totalRecords),
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
func (m PlansTableModel) SelectedIndex() int {
	return m.selected
}

// ShouldShowDetails returns if should show details
func (m PlansTableModel) ShouldShowDetails() bool {
	return m.showDetails
}

// RunPlansTable runs an interactive table for plans with navigation to details
func RunPlansTable(title string, columns []table.Column, rows []table.Row, originalPlans []map[string]string) error {
	if len(rows) == 0 {
		fmt.Println(ErrorStyle.Render("\nNo plans found matching your filters.\n"))
		return nil
	}

	for {
		p := tea.NewProgram(
			NewPlansTable(title, columns, rows, originalPlans),
			tea.WithAltScreen(),
		)

		m, err := p.Run()
		if err != nil {
			return err
		}

		if model, ok := m.(PlansTableModel); ok {
			if model.ShouldShowDetails() && model.SelectedIndex() < len(originalPlans) {
				selectedPlan := originalPlans[model.SelectedIndex()]
				planTitle := fmt.Sprintf("Plan Details: %s", selectedPlan["SLUG"])
				RunPlanDetails(planTitle, selectedPlan)

				continue
			}
		}

		break
	}

	return nil
}
