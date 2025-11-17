package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ServerDetailsModel struct {
	title    string
	server   map[string]string
	quitting bool
}

func NewServerDetails(title string, server map[string]string) ServerDetailsModel {
	return ServerDetailsModel{
		title:  title,
		server: server,
	}
}

func (m ServerDetailsModel) Init() tea.Cmd {
	return nil
}

func (m ServerDetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c", "backspace":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ServerDetailsModel) View() string {
	if m.quitting {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1).
		Padding(0, 1)

	fieldLabelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Width(20).
		Align(lipgloss.Right)

	fieldValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		MarginTop(1)

	// Render fields
	var fields []string

	// Define preferred order of fields
	preferredOrder := []string{
		"ID",
		"Hostname",
		"Status",
		"IPMI Status",
		"Primary IPV4",
		"Primary IPV6",
		"Plan",
		"OS",
		"Project",
		"Location",
		"Tags",
	}

	// Create a map for quick verification
	orderedFields := make(map[string]bool)
	for _, key := range preferredOrder {
		orderedFields[key] = true
	}

	// Render fields in preferred order
	for _, key := range preferredOrder {
		if value, exists := m.server[key]; exists && value != "" {
			line := lipgloss.JoinHorizontal(
				lipgloss.Top,
				fieldLabelStyle.Render(key+":"),
				fieldValueStyle.Render(value),
			)
			fields = append(fields, line)
		}
	}

	// Render remaining fields
	for key, value := range m.server {
		if !orderedFields[key] && value != "" {
			line := lipgloss.JoinHorizontal(
				lipgloss.Top,
				fieldLabelStyle.Render(key+":"),
				fieldValueStyle.Render(value),
			)
			fields = append(fields, line)
		}
	}

	content := strings.Join(fields, "\n")

	box := boxStyle.Render(content)

	help := HelpStyle.Render("esc/backspace: back • q: quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(m.title),
		box,
		"\n",
		help,
	)
}

// RunServerDetails shows the details of a server
func RunServerDetails(title string, server map[string]string) error {
	p := tea.NewProgram(
		NewServerDetails(title, server),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
