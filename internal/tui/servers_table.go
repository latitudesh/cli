package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServersTableModel is a specialized table for servers
type ServersTableModel struct {
	table           table.Model
	totalRecords    int
	title           string
	quitting        bool
	selected        int
	showDetails     bool
	originalServers []map[string]string
}

// NewServersTable creates an interactive table for servers
func NewServersTable(title string, columns []table.Column, rows []table.Row, originalServers []map[string]string) ServersTableModel {
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

	return ServersTableModel{
		table:           t,
		totalRecords:    len(rows),
		title:           title,
		originalServers: originalServers,
	}
}

func (m ServersTableModel) Init() tea.Cmd {
	return nil
}

func (m ServersTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ServersTableModel) View() string {
	if m.quitting {
		return ""
	}

	header := ""
	if m.title != "" {
		header = TitleStyle.Render(m.title) + "\n\n"
	}

	tableView := baseTableStyle.Render(m.table.View())

	footer := footerStyle.Render(
		fmt.Sprintf("Total: %d servers", m.totalRecords),
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
func (m ServersTableModel) SelectedIndex() int {
	return m.selected
}

// ShouldShowDetails returns if should show details
func (m ServersTableModel) ShouldShowDetails() bool {
	return m.showDetails
}

// RunServersTable runs an interactive table for servers with navigation to details
func RunServersTable(title string, columns []table.Column, rows []table.Row, originalServers []map[string]string) error {
	if len(rows) == 0 {
		fmt.Println(ErrorStyle.Render("\nNo servers found.\n"))
		return nil
	}

	for {
		p := tea.NewProgram(
			NewServersTable(title, columns, rows, originalServers),
			tea.WithAltScreen(),
		)

		m, err := p.Run()
		if err != nil {
			return err
		}

		if model, ok := m.(ServersTableModel); ok {
			if model.ShouldShowDetails() && model.SelectedIndex() < len(originalServers) {
				selectedServer := originalServers[model.SelectedIndex()]
				serverTitle := fmt.Sprintf("Server Details: %s", selectedServer["Hostname"])
				RunServerDetails(serverTitle, selectedServer)

				continue
			}
		}

		break
	}

	return nil
}
